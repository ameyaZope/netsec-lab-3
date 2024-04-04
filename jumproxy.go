package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"sync"
)

func main() {
	var keyFileName = flag.String("k", "mykey", "Use the ASCII text passphrase contained in <pwdfile>")
	var listenPort = flag.Int("l", -1, "Reverse-proxy mode: listen for inbound connections on <listenport> and relay them to <destination>:<port>")

	flag.Parse()

	var destHost = "NA"
	var destPort = int64(-1)
	var err error = nil
	if len(flag.Args()) >= 2 {
		destHost = flag.Args()[0]
		destPort, err = strconv.ParseInt(flag.Args()[1], 10, 64)
		if err != nil {
			fmt.Printf("Input Port is not a valid 64 bit number\n")
		}
		fmt.Printf("Destination Host : %s\nDestination Port : %d\n", destHost, destPort)
	}

	fmt.Printf("KeyFileName : %s\n", *keyFileName)
	if *listenPort == -1 {
		fmt.Printf("Client Mode\n")
	} else {
		listener, err := net.Listen("tcp", ":"+fmt.Sprintf("%d", *listenPort))
		if err != nil {
			fmt.Println("Error:", err)
			return
		}
		defer listener.Close()

		fmt.Printf("Server Mode: listening on port %d\n", *listenPort)

		for {
			// Accept incoming connections
			conn, err := listener.Accept()
			if err != nil {
				fmt.Println("Error:", err)
				continue
			}

			// Handle client connection in a goroutine
			go handleClient(conn, destHost, destPort)
		}
	}
}

func handleClient(clientConn net.Conn, destHost string, destPort int64) {
	serviceConn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", destHost, destPort))
	if err != nil {
		log.Fatalf("Failed to connect to service: %v", err)
	}
	defer serviceConn.Close()
	// Forward data from client to service
	var wg sync.WaitGroup
	wg.Add(2)
	// Forward data from client to service
	go func() {
		defer wg.Done()
		_, err := io.Copy(serviceConn, clientConn)
		if err != nil {
			log.Printf("Error relaying data from client to service: %v", err)
		}
	}()

	// Forward data from service to client
	go func() {
		defer wg.Done()
		_, err := io.Copy(clientConn, serviceConn)
		if err != nil {
			log.Printf("Error relaying data from service to client: %v", err)
		}

	}()

	wg.Wait()

	clientConn.Close()
}
