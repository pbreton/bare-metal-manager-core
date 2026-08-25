// SPDX-FileCopyrightText: Copyright (c) 2026 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// dps-tls-proxy provides test-only TLS termination in front of the DPS 0.8.0
// simulator, whose in-cluster gRPC listener is plaintext.
package main

import (
	"crypto/tls"
	"errors"
	"io"
	"log"
	"net"
	"os"
	"time"
)

const dialTimeout = 10 * time.Second

func proxyConnection(client net.Conn, upstreamAddress string) {
	defer client.Close()

	upstream, err := net.DialTimeout("tcp", upstreamAddress, dialTimeout)
	if err != nil {
		log.Printf("connect to DPS upstream: %v", err)
		return
	}
	defer upstream.Close()

	copyDone := make(chan struct{}, 2)
	copyStream := func(destination, source net.Conn) {
		_, copyErr := io.Copy(destination, source)
		if copyErr != nil && !errors.Is(copyErr, net.ErrClosed) {
			log.Printf("proxy DPS stream: %v", copyErr)
		}
		copyDone <- struct{}{}
	}
	go copyStream(upstream, client)
	go copyStream(client, upstream)
	<-copyDone
}

func serve(listener net.Listener, upstreamAddress string) error {
	for {
		connection, err := listener.Accept()
		if err != nil {
			return err
		}
		go proxyConnection(connection, upstreamAddress)
	}
}

func requiredEnv(name string) string {
	value := os.Getenv(name)
	if value == "" {
		log.Fatalf("%s is required", name)
	}
	return value
}

func main() {
	listenAddress := requiredEnv("LISTEN_ADDRESS")
	upstreamAddress := requiredEnv("UPSTREAM_ADDRESS")
	certificate, err := tls.LoadX509KeyPair(requiredEnv("TLS_CERT_FILE"), requiredEnv("TLS_KEY_FILE"))
	if err != nil {
		log.Fatalf("load DPS proxy TLS key pair: %v", err)
	}

	listener, err := tls.Listen("tcp", listenAddress, &tls.Config{
		Certificates: []tls.Certificate{certificate},
		MinVersion:   tls.VersionTLS12,
	})
	if err != nil {
		log.Fatalf("listen for DPS TLS connections: %v", err)
	}
	defer listener.Close()

	log.Printf("proxying DPS TLS traffic from %s to %s", listenAddress, upstreamAddress)
	err = serve(listener, upstreamAddress)
	if err != nil {
		log.Fatalf("serve DPS TLS proxy: %v", err)
	}
}
