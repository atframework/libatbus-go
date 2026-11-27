"""Check C++ test registrations, shared fixtures and mapped Go test results."""

import argparse
import json
from pathlib import Path
import re
import sys


MODULE = Path(__file__).resolve().parents[1]
CASE = re.compile(r"^\s*CASE_TEST\(\s*(\w+)\s*,\s*(\w+)\s*\)", re.MULTILINE)
TEST = re.compile(r"^func (Test\w+)\(.*?\*testing\.T\)", re.MULTILINE)


def cpp_cases(root):
    case_dir = root / "test" / "case"
    if not case_dir.is_dir():
        raise ValueError(f"missing C++ test directory: {case_dir}")
    cases = set()
    excluded = set()
    for source in case_dir.rglob("*"):
        if source.suffix not in {".cpp", ".cc", ".cxx", ".c", ".hpp", ".h"}:
            continue
        content = source.read_text(encoding="utf-8-sig")
        for item in CASE.finditer(content):
            case = ".".join(item.groups())
            if source.name in {"channel_mem_test.cpp", "channel_shm_test.cpp"} or case in {
                "atbus_node_reg.mem_and_send", "atbus_node_reg.shm_and_send"
            }:
                excluded.add(case)
                continue
            if case == "atbus_node_rela.basic_test" and re.search(
                r"^#if\s+0[^\n]*\n\s*\Z", content[:item.start()], re.MULTILINE
            ):
                excluded.add(case)
                continue
            if case in cases:
                raise ValueError(f"duplicate C++ case: {case}")
            cases.add(case)
    if not cases:
        raise ValueError("no C++ test registrations found")
    return cases, excluded


def fixture_differences(cpp_root):
    problems = []
    for local, reference in [
        ("impl/testdata", "atbus_connection_context_enc_dec"),
        ("message_handle/testdata", "atbus_access_data_crosslang"),
    ]:
        left, right = MODULE / local, cpp_root / "test" / "case" / reference
        names = {p.name for directory in (left, right) for p in directory.iterdir()
                 if p.suffix in {".json", ".bytes"}}
        for name in sorted(names):
            source, copy = right / name, left / name
            if not source.is_file() or not copy.is_file():
                problems.append(f"unpaired fixture: {reference}/{name}")
                continue
            if name.endswith(".json"):
                matches = json.loads(source.read_bytes()) == json.loads(copy.read_bytes())
            else:
                matches = source.read_bytes() == copy.read_bytes()
            if not matches:
                problems.append(f"fixture differs: {reference}/{name}")
    return problems


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("cpp_root", type=Path)
    parser.add_argument("--results", nargs="+", type=Path, help="one or more go test -json logs")
    args = parser.parse_args()
    cases, excluded = cpp_cases(args.cpp_root)
    problems = fixture_differences(args.cpp_root)
    rows = {}
    mapping = Path(__file__).with_name("cpp_test_cases.tsv")
    for number, line in enumerate(mapping.read_text(encoding="utf-8").splitlines(), 1):
        if not line or line.startswith("#"):
            continue
        case, *tests = line.split("\t")
        if not re.fullmatch(r"\w+\.\w+", case) or not tests or any(
            not re.fullmatch(r"Test\w+(?:/[\w.+-]+)*", name) for name in tests
        ):
            problems.append(f"{mapping.name}:{number}: invalid mapping")
            continue
        if case in rows or len(tests) != len(set(tests)):
            problems.append(f"{mapping.name}:{number}: duplicate mapping: {case}")
        rows[case] = tests
    problems.extend(f"missing mapping: {name}" for name in sorted(cases - rows.keys()))
    problems.extend(f"obsolete mapping: {name}" for name in sorted(rows.keys() - cases))
    packages = {}
    for source in MODULE.rglob("*_test.go"):
        for test in TEST.findall(source.read_text(encoding="utf-8-sig")):
            package = "github.com/atframework/libatbus-go"
            if source.parent != MODULE:
                package += "/" + source.parent.relative_to(MODULE).as_posix()
            packages.setdefault(test, set()).add(package)

    passed = set()
    skipped = set()
    for path in args.results or []:
        with path.open(encoding="utf-8-sig") as stream:
            for line in stream:
                event = json.loads(line)
                action = event.get("Action")
                key = (event.get("Package"), event.get("Test"))
                if action in {"fail", "build-fail"}:
                    problems.append(f"{path}: {action} {key}")
                if action == "pass":
                    passed.add(key)
                elif action == "skip" and key[1]:
                    skipped.add(key)

    for case, tests in rows.items():
        for test in tests:
            owners = packages.get(test.split("/", 1)[0], set())
            if len(owners) != 1:
                problems.append(f"{case}: unresolved Go test {test}")
                continue
            package = next(iter(owners))
            if args.results and ((package, test) not in passed or (package, None) not in passed):
                problems.append(f"{case}: no completed test and package result for {test}")
            for skipped_package, subtest in skipped - passed:
                if skipped_package == package and subtest.startswith(test + "/"):
                    problems.append(f"{case}: subtest skipped on all supplied platforms: {subtest}")
    for error in problems:
        print(error, file=sys.stderr)
    print(f"{len(cases)} in scope, {len(excluded)} excluded or disabled, "
          f"{len(rows)} mappings, {len(problems)} errors")
    return int(bool(problems))


if __name__ == "__main__":
    sys.exit(main())
