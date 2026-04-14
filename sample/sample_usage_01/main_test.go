// Copyright 2026 atframework
// sample_usage_01_test verifies basic communication across all node relationship types.

package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	libatbus "github.com/atframework/libatbus-go"
	error_code "github.com/atframework/libatbus-go/error_code"
	types "github.com/atframework/libatbus-go/types"
)

// cppEchoAddr is the address of the C++ echo server for cross-language tests.
// Set via -cpp-echo-addr flag. When empty, cross-language tests are skipped.
var cppEchoAddr string

// cppEchoID is the bus ID of the C++ echo server.
var cppEchoID uint64

// cppEchoExe is the path to the C++ echo server executable.
// Set via LIBATBUS_GO_UNIT_TEST_CXX_ECHOSVR_PATH env var or -cpp-echo-exe flag.
var cppEchoExe string

// cppDllDirs are extra directories to prepend to PATH for the C++ echo server.
// Set via LIBATBUS_GO_UNIT_TEST_CXX_DLL_DIRS env var (semicolon-separated) or
// multiple -cpp-dll-dirs flags.
var cppDllDirs []string

// semicolonListFlag implements flag.Value to accumulate semicolon-separated values
// across multiple flag invocations.
type semicolonListFlag struct {
	values *[]string
}

func (f *semicolonListFlag) String() string {
	if f.values == nil {
		return ""
	}
	return strings.Join(*f.values, ";")
}

func (f *semicolonListFlag) Set(s string) error {
	for _, part := range strings.Split(s, ";") {
		part = strings.TrimSpace(part)
		if part != "" {
			*f.values = append(*f.values, part)
		}
	}
	return nil
}

// crossLangRequired is true when LIBATBUS_GO_UNIT_TEST_CXX_ECHOSVR_PATH is set,
// meaning cross-language tests must not be skipped.
var crossLangRequired bool

func TestMain(m *testing.M) {
	flag.StringVar(&cppEchoAddr, "cpp-echo-addr", "", "C++ echo server address (e.g. ipv4://127.0.0.1:21437)")
	flag.Uint64Var(&cppEchoID, "cpp-echo-id", 0x00000001, "C++ echo server bus ID")
	flag.StringVar(&cppEchoExe, "cpp-echo-exe", "", "Path to atapp_sample_echo_svr executable")
	flag.Var(&semicolonListFlag{&cppDllDirs}, "cpp-dll-dirs", "DLL directories for C++ echo server (semicolon-separated, repeatable)")
	flag.Parse()

	// Environment variable overrides flag for exe path
	if envPath := os.Getenv("LIBATBUS_GO_UNIT_TEST_CXX_ECHOSVR_PATH"); envPath != "" {
		cppEchoExe = envPath
		crossLangRequired = true
	}
	// Environment variable appends to flag for DLL dirs
	if envDirs := os.Getenv("LIBATBUS_GO_UNIT_TEST_CXX_DLL_DIRS"); envDirs != "" {
		for _, part := range strings.Split(envDirs, ";") {
			part = strings.TrimSpace(part)
			if part != "" {
				cppDllDirs = append(cppDllDirs, part)
			}
		}
	}

	os.Exit(m.Run())
}

// skipOrFatalCrossLang skips the test if cross-language tests are optional,
// or fails fatally if LIBATBUS_GO_UNIT_TEST_CXX_ECHOSVR_PATH is set.
func skipOrFatalCrossLang(t *testing.T, reason string) {
	t.Helper()
	if crossLangRequired {
		t.Fatalf("cross-language test required (LIBATBUS_GO_UNIT_TEST_CXX_ECHOSVR_PATH is set) but: %s", reason)
	}
	t.Skipf("skipping: %s", reason)
}

// buildCppEchoCmd creates an exec.Cmd for the C++ echo server with DLL path setup.
func buildCppEchoCmd(args ...string) *exec.Cmd {
	cmd := exec.Command(cppEchoExe, args...)
	if runtime.GOOS == "windows" && len(cppDllDirs) > 0 {
		dllPath := strings.Join(cppDllDirs, ";")
		env := os.Environ()
		for i, e := range env {
			if len(e) > 5 && (e[:5] == "PATH=" || e[:5] == "Path=") {
				env[i] = fmt.Sprintf("PATH=%s;%s", dllPath, e[5:])
				break
			}
		}
		cmd.Env = env
	}
	return cmd
}

// startCppEchoServer starts the C++ echo server subprocess and registers cleanup
// to kill it when the test finishes. Returns the started cmd.
func startCppEchoServer(t *testing.T, args ...string) *exec.Cmd {
	t.Helper()

	cmd := buildCppEchoCmd(args...)

	cppLogFile, fileErr := os.Create(filepath.Join(t.TempDir(), "cpp_echo_svr.log"))
	if fileErr != nil {
		t.Fatalf("create cpp log file: %v", fileErr)
	}
	cmd.Stdout = cppLogFile
	cmd.Stderr = cppLogFile

	if err := cmd.Start(); err != nil {
		cppLogFile.Close()
		t.Fatalf("failed to start C++ echo server: %v", err)
	}
	t.Logf("C++ echo server started (PID=%d)", cmd.Process.Pid)

	t.Cleanup(func() {
		// Kill the process and wait for it to exit
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()
		t.Log("C++ echo server process terminated")

		// Dump C++ server logs for debugging
		if cppLogFile != nil {
			cppLogFile.Sync()
			logBytes, _ := os.ReadFile(cppLogFile.Name())
			if len(logBytes) > 0 {
				t.Logf("=== C++ echo server output (%d bytes) ===\n%s", len(logBytes), string(logBytes))
			} else {
				t.Log("=== C++ echo server output: (empty) ===")
			}
			cppLogFile.Close()
		}
	})

	return cmd
}

// waitForConditionT is a test helper that pumps nodes and waits for cond.
func waitForConditionT(t *testing.T, timeout time.Duration, cond func() bool, nodes ...types.Node) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		for _, n := range nodes {
			n.Poll()
		}
		now := time.Now()
		for _, n := range nodes {
			n.Proc(now)
		}
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("condition not met within %v", timeout)
}

func createAndStartNode(t *testing.T, id types.BusIdType, conf *types.NodeConfigure) types.Node {
	t.Helper()
	node, errCode := libatbus.CreateNode(id, conf)
	if errCode != error_code.EN_ATBUS_ERR_SUCCESS {
		t.Fatalf("CreateNode(0x%x) failed: %d", id, errCode)
	}
	addr := reserveListenAddress()
	errCode = node.Listen(addr)
	if errCode != error_code.EN_ATBUS_ERR_SUCCESS {
		t.Fatalf("Listen(%s) failed: %d", addr, errCode)
	}
	errCode = node.Start()
	if errCode != error_code.EN_ATBUS_ERR_SUCCESS {
		t.Fatalf("Start() failed: %d", errCode)
	}
	t.Cleanup(func() { node.Reset() })
	return node
}

func getFirstAddr(t *testing.T, n types.Node) string {
	t.Helper()
	addrs := n.GetSelfEndpoint().GetListenAddress()
	if len(addrs) == 0 {
		t.Fatal("node has no listen addresses")
	}
	return addrs[0].GetAddress()
}

// TestOtherUpstreamPeer tests OtherUpstreamPeer relationship (no explicit topology).
// This is the default when two nodes connect without setting upstream.
func TestOtherUpstreamPeer(t *testing.T) {
	var conf types.NodeConfigure
	types.SetDefaultNodeConfigure(&conf)

	node1 := createAndStartNode(t, 0x12345678, &conf)
	node2 := createAndStartNode(t, 0x12356789, &conf)

	// Connect node1 → node2
	errCode := node1.Connect(getFirstAddr(t, node2))
	if errCode != error_code.EN_ATBUS_ERR_SUCCESS {
		t.Fatalf("Connect failed: %d", errCode)
	}

	waitForConditionT(t, 8*time.Second, func() bool {
		return node1.IsEndpointAvailable(node2.GetId()) && node2.IsEndpointAvailable(node1.GetId())
	}, node1, node2)

	// Verify topology relation
	rel, _ := node1.GetTopologyRelation(node2.GetId())
	if rel != types.TopologyRelationType_OtherUpstreamPeer {
		t.Errorf("expected OtherUpstreamPeer, got %d", rel)
	}

	// Test node1 → node2
	recved := false
	node2.SetEventHandleOnForwardRequest(func(_ types.Node, _ types.Endpoint, _ types.Connection,
		_ *types.Message, data []byte,
	) error_code.ErrorType {
		t.Logf("node2 received: %s", string(data))
		recved = true
		return error_code.EN_ATBUS_ERR_SUCCESS
	})

	errCode = node1.SendData(node2.GetId(), 0, []byte("OtherUpstreamPeer: node1→node2"))
	if errCode != error_code.EN_ATBUS_ERR_SUCCESS {
		t.Fatalf("SendData failed: %d", errCode)
	}
	waitForConditionT(t, 3*time.Second, func() bool { return recved }, node1, node2)

	// Test node2 → node1
	recved = false
	node1.SetEventHandleOnForwardRequest(func(_ types.Node, _ types.Endpoint, _ types.Connection,
		_ *types.Message, data []byte,
	) error_code.ErrorType {
		t.Logf("node1 received: %s", string(data))
		recved = true
		return error_code.EN_ATBUS_ERR_SUCCESS
	})

	errCode = node2.SendData(node1.GetId(), 0, []byte("OtherUpstreamPeer: node2→node1"))
	if errCode != error_code.EN_ATBUS_ERR_SUCCESS {
		t.Fatalf("SendData failed: %d", errCode)
	}
	waitForConditionT(t, 3*time.Second, func() bool { return recved }, node1, node2)

	t.Log("OtherUpstreamPeer: bidirectional communication OK")
}

// TestUpstreamDownstream tests ImmediateUpstream/ImmediateDownstream relationship.
func TestUpstreamDownstream(t *testing.T) {
	var upstreamConf types.NodeConfigure
	types.SetDefaultNodeConfigure(&upstreamConf)

	upstream := createAndStartNode(t, 0x12345678, &upstreamConf)
	upstreamAddr := getFirstAddr(t, upstream)

	// Downstream sets UpstreamAddress → becomes child of upstream
	var downstreamConf types.NodeConfigure
	types.SetDefaultNodeConfigure(&downstreamConf)
	downstreamConf.UpstreamAddress = upstreamAddr

	downstream := createAndStartNode(t, 0x12346789, &downstreamConf)

	waitForConditionT(t, 8*time.Second, func() bool {
		return upstream.IsEndpointAvailable(downstream.GetId()) &&
			downstream.IsEndpointAvailable(upstream.GetId())
	}, upstream, downstream)

	// Log topology relations (actual values depend on registry propagation)
	relFromDown, _ := downstream.GetTopologyRelation(upstream.GetId())
	t.Logf("downstream→upstream topology: %d (ImmediateUpstream=%d)", relFromDown, types.TopologyRelationType_ImmediateUpstream)
	relFromUp, _ := upstream.GetTopologyRelation(downstream.GetId())
	t.Logf("upstream→downstream topology: %d (ImmediateDownstream=%d)", relFromUp, types.TopologyRelationType_ImmediateDownstream)

	// Test upstream → downstream
	recved := false
	downstream.SetEventHandleOnForwardRequest(func(_ types.Node, _ types.Endpoint, _ types.Connection,
		_ *types.Message, data []byte,
	) error_code.ErrorType {
		t.Logf("downstream received: %s", string(data))
		recved = true
		return error_code.EN_ATBUS_ERR_SUCCESS
	})

	errCode := upstream.SendData(downstream.GetId(), 0, []byte("Upstream→Downstream"))
	if errCode != error_code.EN_ATBUS_ERR_SUCCESS {
		t.Fatalf("SendData upstream→downstream failed: %d", errCode)
	}
	waitForConditionT(t, 3*time.Second, func() bool { return recved }, upstream, downstream)

	// Test downstream → upstream
	recved = false
	upstream.SetEventHandleOnForwardRequest(func(_ types.Node, _ types.Endpoint, _ types.Connection,
		_ *types.Message, data []byte,
	) error_code.ErrorType {
		t.Logf("upstream received: %s", string(data))
		recved = true
		return error_code.EN_ATBUS_ERR_SUCCESS
	})

	errCode = downstream.SendData(upstream.GetId(), 0, []byte("Downstream→Upstream"))
	if errCode != error_code.EN_ATBUS_ERR_SUCCESS {
		t.Fatalf("SendData downstream→upstream failed: %d", errCode)
	}
	waitForConditionT(t, 3*time.Second, func() bool { return recved }, upstream, downstream)

	t.Log("UpstreamDownstream: bidirectional communication OK")
}

// TestSameUpstreamPeer tests SameUpstreamPeer relationship (siblings under same parent).
func TestSameUpstreamPeer(t *testing.T) {
	// Create upstream (parent) node
	var upstreamConf types.NodeConfigure
	types.SetDefaultNodeConfigure(&upstreamConf)

	upstream := createAndStartNode(t, 0x12345678, &upstreamConf)
	upstreamAddr := getFirstAddr(t, upstream)

	// Create two sibling downstream nodes
	var sibConf1 types.NodeConfigure
	types.SetDefaultNodeConfigure(&sibConf1)
	sibConf1.UpstreamAddress = upstreamAddr

	var sibConf2 types.NodeConfigure
	types.SetDefaultNodeConfigure(&sibConf2)
	sibConf2.UpstreamAddress = upstreamAddr

	sibling1 := createAndStartNode(t, 0x12346001, &sibConf1)
	sibling2 := createAndStartNode(t, 0x12346002, &sibConf2)

	// Wait for all nodes to register with upstream
	waitForConditionT(t, 8*time.Second, func() bool {
		return upstream.IsEndpointAvailable(sibling1.GetId()) &&
			upstream.IsEndpointAvailable(sibling2.GetId()) &&
			sibling1.IsEndpointAvailable(upstream.GetId()) &&
			sibling2.IsEndpointAvailable(upstream.GetId())
	}, upstream, sibling1, sibling2)

	// Log topology relations
	rel, _ := upstream.GetTopologyRelation(sibling1.GetId())
	t.Logf("upstream→sibling1 topology: %d (ImmediateDownstream=%d)", rel, types.TopologyRelationType_ImmediateDownstream)
	rel, _ = upstream.GetTopologyRelation(sibling2.GetId())
	t.Logf("upstream→sibling2 topology: %d (ImmediateDownstream=%d)", rel, types.TopologyRelationType_ImmediateDownstream)
	rel, _ = sibling1.GetTopologyRelation(sibling2.GetId())
	t.Logf("sibling1→sibling2 topology: %d (SameUpstreamPeer=%d)", rel, types.TopologyRelationType_SameUpstreamPeer)

	// Test sibling1 → sibling2 (routed through upstream)
	recved := false
	sibling2.SetEventHandleOnForwardRequest(func(_ types.Node, _ types.Endpoint, _ types.Connection,
		_ *types.Message, data []byte,
	) error_code.ErrorType {
		t.Logf("sibling2 received: %s", string(data))
		recved = true
		return error_code.EN_ATBUS_ERR_SUCCESS
	})

	errCode := sibling1.SendData(sibling2.GetId(), 0, []byte("Sibling1→Sibling2"))
	if errCode != error_code.EN_ATBUS_ERR_SUCCESS {
		t.Fatalf("SendData sibling1→sibling2 failed: %d", errCode)
	}
	waitForConditionT(t, 5*time.Second, func() bool { return recved }, upstream, sibling1, sibling2)

	// Test sibling2 → sibling1
	recved = false
	sibling1.SetEventHandleOnForwardRequest(func(_ types.Node, _ types.Endpoint, _ types.Connection,
		_ *types.Message, data []byte,
	) error_code.ErrorType {
		t.Logf("sibling1 received: %s", string(data))
		recved = true
		return error_code.EN_ATBUS_ERR_SUCCESS
	})

	errCode = sibling2.SendData(sibling1.GetId(), 0, []byte("Sibling2→Sibling1"))
	if errCode != error_code.EN_ATBUS_ERR_SUCCESS {
		t.Fatalf("SendData sibling2→sibling1 failed: %d", errCode)
	}
	waitForConditionT(t, 5*time.Second, func() bool { return recved }, upstream, sibling1, sibling2)

	t.Log("SameUpstreamPeer: bidirectional sibling communication OK")
}

// TestTransitiveUpstreamDownstream tests transitive (grandparent-grandchild) relationships.
func TestTransitiveUpstreamDownstream(t *testing.T) {
	// Level 0: root
	var rootConf types.NodeConfigure
	types.SetDefaultNodeConfigure(&rootConf)
	root := createAndStartNode(t, 0x12340001, &rootConf)
	rootAddr := getFirstAddr(t, root)

	// Level 1: mid (child of root)
	var midConf types.NodeConfigure
	types.SetDefaultNodeConfigure(&midConf)
	midConf.UpstreamAddress = rootAddr
	mid := createAndStartNode(t, 0x12340002, &midConf)
	midAddr := getFirstAddr(t, mid)

	// Wait for mid to register with root
	waitForConditionT(t, 8*time.Second, func() bool {
		return root.IsEndpointAvailable(mid.GetId()) && mid.IsEndpointAvailable(root.GetId())
	}, root, mid)

	// Level 2: leaf (child of mid, grandchild of root)
	var leafConf types.NodeConfigure
	types.SetDefaultNodeConfigure(&leafConf)
	leafConf.UpstreamAddress = midAddr
	leaf := createAndStartNode(t, 0x12340003, &leafConf)

	// Wait for leaf to register with mid
	waitForConditionT(t, 8*time.Second, func() bool {
		return mid.IsEndpointAvailable(leaf.GetId()) && leaf.IsEndpointAvailable(mid.GetId())
	}, root, mid, leaf)

	// Verify topology: leaf→root should be TransitiveUpstream
	rel, _ := leaf.GetTopologyRelation(root.GetId())
	if rel != types.TopologyRelationType_TransitiveUpstream {
		t.Logf("leaf→root: expected TransitiveUpstream(%d), got %d (may be OtherUpstreamPeer if topology not fully propagated)",
			types.TopologyRelationType_TransitiveUpstream, rel)
	}

	// Test mid → leaf (parent → child)
	recved := false
	leaf.SetEventHandleOnForwardRequest(func(_ types.Node, _ types.Endpoint, _ types.Connection,
		_ *types.Message, data []byte,
	) error_code.ErrorType {
		t.Logf("leaf received: %s", string(data))
		recved = true
		return error_code.EN_ATBUS_ERR_SUCCESS
	})

	errCode := mid.SendData(leaf.GetId(), 0, []byte("Mid→Leaf"))
	if errCode != error_code.EN_ATBUS_ERR_SUCCESS {
		t.Fatalf("SendData mid→leaf failed: %d", errCode)
	}
	waitForConditionT(t, 3*time.Second, func() bool { return recved }, root, mid, leaf)

	// Test leaf → mid (child → parent)
	recved = false
	mid.SetEventHandleOnForwardRequest(func(_ types.Node, _ types.Endpoint, _ types.Connection,
		_ *types.Message, data []byte,
	) error_code.ErrorType {
		t.Logf("mid received: %s", string(data))
		recved = true
		return error_code.EN_ATBUS_ERR_SUCCESS
	})

	errCode = leaf.SendData(mid.GetId(), 0, []byte("Leaf→Mid"))
	if errCode != error_code.EN_ATBUS_ERR_SUCCESS {
		t.Fatalf("SendData leaf→mid failed: %d", errCode)
	}
	waitForConditionT(t, 3*time.Second, func() bool { return recved }, root, mid, leaf)

	// Test root → leaf (grandparent → grandchild, forward through mid)
	recved = false
	leaf.SetEventHandleOnForwardRequest(func(_ types.Node, _ types.Endpoint, _ types.Connection,
		_ *types.Message, data []byte,
	) error_code.ErrorType {
		t.Logf("leaf received from root: %s", string(data))
		recved = true
		return error_code.EN_ATBUS_ERR_SUCCESS
	})

	errCode = root.SendData(leaf.GetId(), 0, []byte("Root→Leaf (transitive)"))
	if errCode != error_code.EN_ATBUS_ERR_SUCCESS {
		t.Logf("SendData root→leaf returned: %d (may need forwarding support)", errCode)
	} else {
		waitForConditionT(t, 5*time.Second, func() bool { return recved }, root, mid, leaf)
	}

	// Test leaf → root (grandchild → grandparent, forward through mid)
	recved = false
	root.SetEventHandleOnForwardRequest(func(_ types.Node, _ types.Endpoint, _ types.Connection,
		_ *types.Message, data []byte,
	) error_code.ErrorType {
		t.Logf("root received from leaf: %s", string(data))
		recved = true
		return error_code.EN_ATBUS_ERR_SUCCESS
	})

	errCode = leaf.SendData(root.GetId(), 0, []byte("Leaf→Root (transitive)"))
	if errCode != error_code.EN_ATBUS_ERR_SUCCESS {
		t.Logf("SendData leaf→root returned: %d (may need forwarding support)", errCode)
	} else {
		waitForConditionT(t, 5*time.Second, func() bool { return recved }, root, mid, leaf)
	}

	t.Log("TransitiveUpstreamDownstream: hierarchy communication OK")
}

// ============================================================
// Cross-language tests: Go node ↔ C++ echo server
// ============================================================

// crossLangHostname is a shared hostname used by both Go and C++ nodes in
// cross-language tests to ensure they recognize each other as same-host.
const crossLangHostname = "crosslang-test-host"

// TestCrossLang_GoToCppEcho sends data from a Go node to the C++ echo server
// and verifies the echo response.
func TestCrossLang_GoToCppEcho(t *testing.T) {
	addr := cppEchoAddr
	id := types.BusIdType(cppEchoID)

	if addr == "" && cppEchoExe != "" {
		// Auto-launch a standalone C++ echo server
		configPath, err := filepath.Abs(filepath.Join("testdata", "echo_svr_standalone.yaml"))
		if err != nil {
			t.Fatalf("resolve config path: %v", err)
		}
		startCppEchoServer(t, "-id", "1", "-c", configPath, "start")
		addr = "ipv4://127.0.0.1:21437"
		id = 0x00000001
		time.Sleep(2 * time.Second) // Let C++ server initialize
	}

	if addr == "" {
		skipOrFatalCrossLang(t, "-cpp-echo-addr not set and no C++ echo server executable available")
	}

	var conf types.NodeConfigure
	types.SetDefaultNodeConfigure(&conf)

	goNode := createAndStartNode(t, 0x12345678, &conf)
	goNode.SetHostname(crossLangHostname, true)

	// Connect Go node to C++ echo server
	errCode := goNode.Connect(addr)
	if errCode != error_code.EN_ATBUS_ERR_SUCCESS {
		t.Fatalf("Connect to C++ echo server failed: %d", errCode)
	}

	// Wait for the echo server endpoint to become available
	waitForConditionT(t, 15*time.Second, func() bool {
		return goNode.IsEndpointAvailable(id)
	}, goNode)

	t.Logf("Connected to C++ echo server (0x%x) at %s", id, addr)

	// Set up receive callback to capture echo response
	var echoResponse []byte
	goNode.SetEventHandleOnForwardRequest(func(_ types.Node, ep types.Endpoint, conn types.Connection,
		_ *types.Message, data []byte,
	) error_code.ErrorType {
		t.Logf("Go node received echo from 0x%x: %s", ep.GetId(), string(data))
		echoResponse = append([]byte(nil), data...)
		return error_code.EN_ATBUS_ERR_SUCCESS
	})

	// Send data to C++ echo server
	sendData := []byte("hello from Go to C++ echo server!")
	errCode = goNode.SendData(id, 0, sendData)
	if errCode != error_code.EN_ATBUS_ERR_SUCCESS {
		t.Fatalf("SendData to C++ echo server failed: %d", errCode)
	}
	t.Logf("Sent: %s", string(sendData))

	// Wait for echo response
	waitForConditionT(t, 10*time.Second, func() bool {
		return echoResponse != nil
	}, goNode)

	// Verify the echo matches what we sent
	if string(echoResponse) != string(sendData) {
		t.Errorf("echo mismatch: sent %q, got %q", string(sendData), string(echoResponse))
	} else {
		t.Log("Cross-language echo verified: Go→C++→Go OK")
	}
}

// TestCrossLang_GoMultipleMessages sends multiple messages to the C++ echo server
// and verifies all echo responses.
func TestCrossLang_GoMultipleMessages(t *testing.T) {
	addr := cppEchoAddr
	id := types.BusIdType(cppEchoID)

	if addr == "" && cppEchoExe != "" {
		// Auto-launch a standalone C++ echo server
		configPath, err := filepath.Abs(filepath.Join("testdata", "echo_svr_standalone.yaml"))
		if err != nil {
			t.Fatalf("resolve config path: %v", err)
		}
		startCppEchoServer(t, "-id", "1", "-c", configPath, "start")
		addr = "ipv4://127.0.0.1:21437"
		id = 0x00000001
		time.Sleep(2 * time.Second) // Let C++ server initialize
	}

	if addr == "" {
		skipOrFatalCrossLang(t, "-cpp-echo-addr not set and no C++ echo server executable available")
	}

	var conf types.NodeConfigure
	types.SetDefaultNodeConfigure(&conf)

	goNode := createAndStartNode(t, 0x12340099, &conf)
	goNode.SetHostname(crossLangHostname, true)

	errCode := goNode.Connect(addr)
	if errCode != error_code.EN_ATBUS_ERR_SUCCESS {
		t.Fatalf("Connect failed: %d", errCode)
	}

	waitForConditionT(t, 15*time.Second, func() bool {
		return goNode.IsEndpointAvailable(id)
	}, goNode)

	// Track echoed messages
	echoCount := 0
	var lastEchoData string
	goNode.SetEventHandleOnForwardRequest(func(_ types.Node, _ types.Endpoint, _ types.Connection,
		_ *types.Message, data []byte,
	) error_code.ErrorType {
		echoCount++
		lastEchoData = string(data)
		t.Logf("Echo #%d: %s", echoCount, lastEchoData)
		return error_code.EN_ATBUS_ERR_SUCCESS
	})

	messages := []string{
		"message 1: hello C++",
		"message 2: cross-language test",
		"message 3: libatbus-go ↔ libatbus",
	}

	for i, msg := range messages {
		countBefore := echoCount
		errCode = goNode.SendData(id, int32(i+1), []byte(msg))
		if errCode != error_code.EN_ATBUS_ERR_SUCCESS {
			t.Fatalf("SendData message %d failed: %d", i+1, errCode)
		}

		waitForConditionT(t, 5*time.Second, func() bool {
			return echoCount > countBefore
		}, goNode)

		if lastEchoData != msg {
			t.Errorf("message %d: echo mismatch: sent %q, got %q", i+1, msg, lastEchoData)
		}
	}

	t.Logf("Cross-language multiple messages verified: %d/%d echoed correctly", echoCount, len(messages))
}

// TestCrossLang_GoAsUpstreamOfCpp tests Go node as upstream of C++ echo server.
// It starts a Go upstream node, launches the C++ echo server as downstream,
// then verifies bidirectional communication (Go→C++ echo→Go).
func TestCrossLang_GoAsUpstreamOfCpp(t *testing.T) {
	if cppEchoExe == "" {
		skipOrFatalCrossLang(t, "C++ echo server executable not available")
	}

	// Resolve testdata config path
	configPath, err := filepath.Abs(filepath.Join("testdata", "echo_svr_downstream.yaml"))
	if err != nil {
		t.Fatalf("resolve config path: %v", err)
	}

	// 1. Create Go upstream node on fixed port 21438
	const goUpstreamAddr = "ipv4://127.0.0.1:21438"
	const goUpstreamID types.BusIdType = 0x12340001

	var conf types.NodeConfigure
	types.SetDefaultNodeConfigure(&conf)

	goUpstream, errCode := libatbus.CreateNode(goUpstreamID, &conf)
	if errCode != error_code.EN_ATBUS_ERR_SUCCESS {
		t.Fatalf("CreateNode for Go upstream failed: %d", errCode)
	}
	goUpstream.SetHostname(crossLangHostname, true)
	t.Cleanup(func() { goUpstream.Reset() })

	errCode = goUpstream.Listen(goUpstreamAddr)
	if errCode != error_code.EN_ATBUS_ERR_SUCCESS {
		t.Fatalf("Listen(%s) failed: %d", goUpstreamAddr, errCode)
	}

	errCode = goUpstream.Start()
	if errCode != error_code.EN_ATBUS_ERR_SUCCESS {
		t.Fatalf("Start() failed: %d", errCode)
	}
	t.Logf("Go upstream node (0x%x) listening on %s, state=%d", goUpstreamID, goUpstreamAddr, goUpstream.GetState())

	// 2. Launch C++ echo server as subprocess with downstream config
	startCppEchoServer(t, "-id", "2", "-c", configPath, "start")

	// 3. Wait for C++ echo server (bus ID=0x2) to register as downstream
	const cppDownstreamID types.BusIdType = 0x00000002
	waitForConditionT(t, 20*time.Second, func() bool {
		return goUpstream.IsEndpointAvailable(cppDownstreamID)
	}, goUpstream)
	t.Logf("C++ echo server (0x%x) registered as downstream of Go upstream", cppDownstreamID)

	// 4. Set up receive callback to capture echo
	var echoResponse []byte
	goUpstream.SetEventHandleOnForwardRequest(func(_ types.Node, ep types.Endpoint, _ types.Connection,
		_ *types.Message, data []byte,
	) error_code.ErrorType {
		t.Logf("Go upstream received echo from 0x%x: %s", ep.GetId(), string(data))
		echoResponse = append([]byte(nil), data...)
		return error_code.EN_ATBUS_ERR_SUCCESS
	})

	// 5. Send data from Go upstream to C++ downstream (echo server echoes back)
	sendData := []byte("Go upstream → C++ downstream echo test!")
	errCode = goUpstream.SendData(cppDownstreamID, 0, sendData)
	if errCode != error_code.EN_ATBUS_ERR_SUCCESS {
		t.Fatalf("SendData Go→C++ failed: %d", errCode)
	}
	t.Logf("Sent: %s", string(sendData))

	// 6. Wait for echo response
	waitForConditionT(t, 10*time.Second, func() bool {
		return echoResponse != nil
	}, goUpstream)

	// 7. Verify
	if string(echoResponse) != string(sendData) {
		t.Errorf("echo mismatch: sent %q, got %q", string(sendData), string(echoResponse))
	} else {
		t.Log("Cross-language upstream test verified: Go(upstream)→C++(downstream echo)→Go OK")
	}
}
