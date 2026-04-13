# libatbus-go - VS Code Copilot Notes

<!-- The main project instructions are in AGENTS.md at the repository root. -->
<!-- This file contains only VS Code Copilot-specific notes (skills, path-specific references). -->
<!-- VS Code Copilot reads both this file and the nearest AGENTS.md automatically. -->

## Skills (How-to playbooks)

Operational, copy/paste-friendly guides live in `.agents/skills/` (cross-client [Agent Skills](https://agentskills.io/) standard):

- Entry point: `.agents/skills/README.md`

| Skill             | Path                                      | Description                                                                            |
| ----------------- | ----------------------------------------- | -------------------------------------------------------------------------------------- |
| Build             | `.agents/skills/build/SKILL.md`           | Build, code generation, dependencies                                                   |
| Testing           | `.agents/skills/testing/SKILL.md`         | Run and write tests, cross-language vectors, concurrency testing                       |
| Protocol & Crypto | `.agents/skills/protocol-crypto/SKILL.md` | ECDH handshake, encryption/compression negotiation, message framing, access token auth |
| Architecture      | `.agents/skills/architecture/SKILL.md`    | Module structure, concurrency model, C++ parity patterns                               |

## Key Rules

- This is a C++ → Go translation project; aim for feature parity with C++ `libatbus`.
- `mem://` and `shm://` channels are **intentionally excluded** from the Go implementation.
- The `Endpoint` struct and `nodeEventTimer.pingList` require `sync.Mutex` — I/O goroutine disconnect callbacks race with the main loop.
- Use snapshot-and-release pattern for mutex usage: lock briefly to copy/clear state, then operate outside the lock.
- Tests use time-driven `Proc(now)` instead of wall-clock waits.
- Follow the parent repo's test instructions: `/.github/instructions/gotest.instructions.md`.
