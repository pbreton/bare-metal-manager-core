// SPDX-FileCopyrightText: Copyright (c) 2026 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"io"
	"net"
	"testing"
	"time"
)

func TestProxyConnection(t *testing.T) {
	upstreamListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen for upstream: %v", err)
	}
	defer upstreamListener.Close()

	go func() {
		connection, acceptErr := upstreamListener.Accept()
		if acceptErr != nil {
			return
		}
		defer connection.Close()
		_, _ = io.Copy(connection, connection)
	}()

	proxySide, testSide := net.Pipe()
	defer testSide.Close()
	go proxyConnection(proxySide, upstreamListener.Addr().String())

	err = testSide.SetDeadline(time.Now().Add(5 * time.Second))
	if err != nil {
		t.Fatalf("set test deadline: %v", err)
	}
	payload := []byte("DPS proxy test")
	_, err = testSide.Write(payload)
	if err != nil {
		t.Fatalf("write through proxy: %v", err)
	}

	response := make([]byte, len(payload))
	_, err = io.ReadFull(testSide, response)
	if err != nil {
		t.Fatalf("read through proxy: %v", err)
	}
	if string(response) != string(payload) {
		t.Fatalf("proxy response = %q, want %q", response, payload)
	}
}
