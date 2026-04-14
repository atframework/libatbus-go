// Copyright 2026 atframework
// sample_usage_01 demonstrates basic two-node communication using libatbus-go.
// This is the Go equivalent of the C++ sample_usage_01.cpp in libatbus.

package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"time"

	libatbus "github.com/atframework/libatbus-go"
	error_code "github.com/atframework/libatbus-go/error_code"
	types "github.com/atframework/libatbus-go/types"
)

func reserveListenAddress() string {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		panic(fmt.Sprintf("Failed to reserve listen address: %v", err))
	}
	addr := listener.Addr().String()
	_ = listener.Close()
	return "ipv4://" + addr
}

func main() {
	// Command line flags for cross-language testing
	addr1 := flag.String("addr1", "", "Listen address for node1 (e.g. ipv4://127.0.0.1:16387)")
	addr2 := flag.String("addr2", "", "Listen address for node2 (e.g. ipv4://127.0.0.1:16388)")
	id1 := flag.Uint64("id1", 0x12345678, "Bus ID for node1")
	id2 := flag.Uint64("id2", 0x12356789, "Bus ID for node2")
	remoteAddr := flag.String("remote", "", "Remote address to connect to (for single-node cross-language mode)")
	localOnly := flag.Bool("local", false, "Run in single-node mode (only create node1, connect to --remote)")
	flag.Parse()

	// Use random ports if not specified
	if *addr1 == "" {
		*addr1 = reserveListenAddress()
	}
	if *addr2 == "" && !*localOnly {
		*addr2 = reserveListenAddress()
	}

	var conf types.NodeConfigure
	types.SetDefaultNodeConfigure(&conf)

	if *localOnly {
		// Single-node mode: create one node, connect to remote (for cross-language testing)
		runSingleNode(types.BusIdType(*id1), *addr1, *remoteAddr, &conf)
	} else {
		// Two-node mode: create two nodes locally (same as C++ sample)
		runTwoNodes(types.BusIdType(*id1), types.BusIdType(*id2), *addr1, *addr2, &conf)
	}
}

func runTwoNodes(busId1, busId2 types.BusIdType, addr1, addr2 string, conf *types.NodeConfigure) {
	// Create two communication nodes
	node1, errCode := libatbus.CreateNode(busId1, conf) // BUS ID=0x12345678
	if errCode != error_code.EN_ATBUS_ERR_SUCCESS {
		fmt.Fprintf(os.Stderr, "Failed to create node1: %d\n", errCode)
		os.Exit(1)
	}
	node2, errCode := libatbus.CreateNode(busId2, conf) // BUS ID=0x12356789
	if errCode != error_code.EN_ATBUS_ERR_SUCCESS {
		fmt.Fprintf(os.Stderr, "Failed to create node2: %d\n", errCode)
		os.Exit(1)
	}
	defer node1.Reset()
	defer node2.Reset()

	// Listen on addresses
	errCode = node1.Listen(addr1)
	if errCode != error_code.EN_ATBUS_ERR_SUCCESS {
		fmt.Fprintf(os.Stderr, "node1 listen failed: %d\n", errCode)
		os.Exit(1)
	}
	errCode = node2.Listen(addr2)
	if errCode != error_code.EN_ATBUS_ERR_SUCCESS {
		fmt.Fprintf(os.Stderr, "node2 listen failed: %d\n", errCode)
		os.Exit(1)
	}

	fmt.Printf("node1 (0x%x) listening on %s\n", busId1, addr1)
	fmt.Printf("node2 (0x%x) listening on %s\n", busId2, addr2)

	// Start nodes
	errCode = node1.Start()
	if errCode != error_code.EN_ATBUS_ERR_SUCCESS {
		fmt.Fprintf(os.Stderr, "node1 start failed: %d\n", errCode)
		os.Exit(1)
	}
	errCode = node2.Start()
	if errCode != error_code.EN_ATBUS_ERR_SUCCESS {
		fmt.Fprintf(os.Stderr, "node2 start failed: %d\n", errCode)
		os.Exit(1)
	}

	// node1 connects to node2
	errCode = node1.Connect(addr2)
	if errCode != error_code.EN_ATBUS_ERR_SUCCESS {
		fmt.Fprintf(os.Stderr, "node1 connect failed: %d\n", errCode)
		os.Exit(1)
	}

	// Wait for connection to be established
	fmt.Println("Waiting for connection...")
	connected := waitForCondition(8*time.Second, func() bool {
		return node1.IsEndpointAvailable(node2.GetId()) && node2.IsEndpointAvailable(node1.GetId())
	}, node1, node2)

	if !connected {
		fmt.Fprintln(os.Stderr, "Failed to establish connection within timeout")
		os.Exit(1)
	}
	fmt.Println("Connection established!")

	// Set up receive callback on node2
	recved := false
	node2.SetEventHandleOnForwardRequest(func(n types.Node, ep types.Endpoint, conn types.Connection,
		msg *types.Message, data []byte,
	) error_code.ErrorType {
		if ep != nil && conn != nil {
			fmt.Printf("atbus node 0x%x receive data from 0x%x(connection: %s): ",
				n.GetId(), ep.GetId(), conn.GetAddress().GetAddress())
		}
		fmt.Println(string(data))
		recved = true
		return error_code.EN_ATBUS_ERR_SUCCESS
	})

	// Send data from node1 to node2
	sendData := "hello world!"
	errCode = node1.SendData(node2.GetId(), 0, []byte(sendData))
	if errCode != error_code.EN_ATBUS_ERR_SUCCESS {
		fmt.Fprintf(os.Stderr, "SendData failed: %d\n", errCode)
		os.Exit(1)
	}
	fmt.Printf("node1 sent: %s\n", sendData)

	// Wait for data to be received
	waitForCondition(8*time.Second, func() bool {
		return recved
	}, node1, node2)

	if !recved {
		fmt.Fprintln(os.Stderr, "Failed to receive data within timeout")
		os.Exit(1)
	}

	fmt.Println("Sample completed successfully!")
}

func runSingleNode(busId types.BusIdType, listenAddr, remoteAddr string, conf *types.NodeConfigure) {
	if remoteAddr == "" {
		fmt.Fprintln(os.Stderr, "--remote is required in single-node (--local) mode")
		os.Exit(1)
	}

	node, errCode := libatbus.CreateNode(busId, conf)
	if errCode != error_code.EN_ATBUS_ERR_SUCCESS {
		fmt.Fprintf(os.Stderr, "Failed to create node: %d\n", errCode)
		os.Exit(1)
	}
	defer node.Reset()

	errCode = node.Listen(listenAddr)
	if errCode != error_code.EN_ATBUS_ERR_SUCCESS {
		fmt.Fprintf(os.Stderr, "node listen failed: %d\n", errCode)
		os.Exit(1)
	}

	fmt.Printf("node (0x%x) listening on %s\n", busId, listenAddr)

	errCode = node.Start()
	if errCode != error_code.EN_ATBUS_ERR_SUCCESS {
		fmt.Fprintf(os.Stderr, "node start failed: %d\n", errCode)
		os.Exit(1)
	}

	// Set up receive callback
	recved := false
	node.SetEventHandleOnForwardRequest(func(n types.Node, ep types.Endpoint, conn types.Connection,
		msg *types.Message, data []byte,
	) error_code.ErrorType {
		if ep != nil && conn != nil {
			fmt.Printf("atbus node 0x%x receive data from 0x%x(connection: %s): ",
				n.GetId(), ep.GetId(), conn.GetAddress().GetAddress())
		}
		fmt.Println(string(data))
		recved = true
		return error_code.EN_ATBUS_ERR_SUCCESS
	})

	// Connect to remote node
	errCode = node.Connect(remoteAddr)
	if errCode != error_code.EN_ATBUS_ERR_SUCCESS {
		fmt.Fprintf(os.Stderr, "node connect failed: %d\n", errCode)
		os.Exit(1)
	}
	fmt.Printf("Connecting to %s...\n", remoteAddr)

	// Run event loop waiting for messages
	fmt.Println("Waiting for messages (press Ctrl+C to quit)...")
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		node.Poll()
		node.Proc(time.Now())
		if recved {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if recved {
		fmt.Println("Received message from remote node!")
	} else {
		fmt.Println("Timeout waiting for messages.")
	}
}

// waitForCondition pumps nodes and waits for a condition to become true.
func waitForCondition(timeout time.Duration, condition func() bool, nodes ...types.Node) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		for _, n := range nodes {
			n.Poll()
		}
		now := time.Now()
		for _, n := range nodes {
			n.Proc(now)
		}
		if condition() {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}
