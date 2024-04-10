package main

import (
	"bufio"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"sync"

	"golang.org/x/crypto/pbkdf2"
)

func generateKey(secret []byte, saltInput []byte) ([]byte, []byte) {
	var salt []byte
	if saltInput == nil {
		salt = make([]byte, 16)
		_, err := rand.Read(salt)
		if err != nil {
			log.Fatalf("Error During Salt Generation for Key Generation %v", err)
		}
	} else {
		salt = saltInput
	}

	key := pbkdf2.Key(secret, salt, 4096, 32, sha256.New) // 256-bit key
	return key, salt
}

func encrypt(plaintext []byte, key []byte) ([]byte, []byte, error) {
	aesCipher, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, err
	}
	gcm, err := cipher.NewGCM(aesCipher)
	if err != nil {
		return nil, nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	_, errRandNonce := io.ReadFull(rand.Reader, nonce)
	if errRandNonce != nil {
		return nil, nil, errRandNonce
	}
	ciphertext := make([]byte, 0, len(plaintext)+gcm.Overhead())
	ciphertext = gcm.Seal(ciphertext, nonce, plaintext, nil)

	return ciphertext, nonce, nil
}

func decrypt(ciphertext []byte, key []byte, nonce []byte) ([]byte, error) {
	aesCipher, errCipher := aes.NewCipher(key)
	if errCipher != nil {
		log.Printf("[decrypt] Calculating Cipher %v\n", errCipher)
		return nil, errCipher
	}
	gcm, errGcm := cipher.NewGCM(aesCipher)
	if errGcm != nil {
		log.Printf("[decrypt] Calculating GCM %v\n", errGcm)
		return nil, errGcm
	}

	var plaintext []byte
	plaintext, errOpen := gcm.Open(plaintext, nonce, ciphertext, nil)
	if errOpen != nil {
		log.Printf("[decrypt] gcm.Open failed %v\n", errOpen)
		log.Fatalf("[decrypt] ciphertext=%x key=%x nonce=%x\n", ciphertext, key, nonce)
		return nil, errOpen
	}
	return plaintext, nil
}

// Encrypts the data and sends it, including the nonce with the ciphertext.
func sendEncrypted(conn net.Conn, data []byte, key []byte) error {
	ciphertext, nonce, errEncrypt := encrypt(data, key)
	if errEncrypt != nil {
		log.Printf("[sendEncrypted] %v", errEncrypt)
		return errEncrypt
	}
	// Assuming a simple protocol: send the nonce assuming 12 byte nonce, cipherTextSize, ciphertext.
	_, errWriteNonce := conn.Write(nonce)
	if errWriteNonce != nil {
		log.Printf("Client [ERROR] Writing Nonce: %v\n", errWriteNonce)
		return errWriteNonce
	}

	cipherTextSize := len(ciphertext)
	sizeBuf := make([]byte, 2)
	binary.BigEndian.PutUint16(sizeBuf, uint16(cipherTextSize))
	_, errWriteCipherTextSize := conn.Write(sizeBuf)
	if errWriteCipherTextSize != nil {
		log.Printf("Client [ERROR] Writing CipherTextSize: %v\n", errWriteCipherTextSize)
	}

	_, errWriteCipherText := conn.Write(ciphertext)
	if errWriteCipherText != nil {
		log.Printf("Client [ERROR] Writing CipherText: %v\n", errWriteCipherText)
		return errWriteCipherText
	}
	return nil
}

// Reads encrypted data, extracts the nonce, and decrypts the data.
func receiveDecrypted(conn net.Conn, key []byte) ([]byte, error) {

	nonce := make([]byte, 12) //assuming that nonce size is 12 bytes
	_, errReadNonce := io.ReadFull(conn, nonce)
	if errReadNonce != nil {
		log.Printf("[recieveDecrypt] Error Reading Nonce : %v\n", errReadNonce)
		return nil, errReadNonce
	}

	cipherTextSizeBuf := make([]byte, 2)
	_, errReadCipherTextSize := io.ReadFull(conn, cipherTextSizeBuf)
	cipherTextSize := uint16(binary.BigEndian.Uint16(cipherTextSizeBuf))
	if errReadCipherTextSize != nil {
		log.Printf("[recieveDecrypt] Error Reading CipherTextSize : %v\n", errReadCipherTextSize)
		return nil, errReadCipherTextSize
	}

	// Assuming the remaining data is the ciphertext. In practice, you might want to prefix the data with its size.
	ciphertext := make([]byte, cipherTextSize)
	_, errReadCipherText := io.ReadFull(conn, ciphertext)
	if errReadCipherText != nil {
		log.Printf("[recieveDecrypt] Error Reading CipherText : %v\n", errReadCipherText)
		return nil, errReadCipherText
	}

	plaintext, errDecrpyt := decrypt(ciphertext, key, nonce)
	if errDecrpyt != nil {
		log.Printf("[recieveDecrypt] Error Decrypt : %v\n", errDecrpyt)
		return nil, errDecrpyt
	}
	return plaintext, nil
}

func main() {
	var keyFileName = flag.String("k", "mykey", "Use the ASCII text passphrase contained in <pwdfile>")
	var listenPort = flag.Int("l", -1, "Reverse-proxy mode: listen for inbound connections on <listenport> and relay them to <destination>:<port>")
	flag.Parse()

	var fileName string
	if *listenPort == -1 {
		fileName = "proxy-client-application.log"
	} else {
		fileName = "proxy-server-application.log"
	}
	logFile, errOpenLogFile := os.OpenFile(fileName, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if errOpenLogFile != nil {
		fmt.Printf("error opening log file: %v", errOpenLogFile)
		os.Exit(1) // Exit if we cannot open the log file
	}
	defer logFile.Close()

	// Set the output of the standard logger to the log file
	log.SetOutput(logFile)

	var destHost = "NA"
	var destPort = int64(-1)
	var err error = nil
	if len(flag.Args()) >= 2 {
		destHost = flag.Args()[0]
		destPort, err = strconv.ParseInt(flag.Args()[1], 10, 64)
		if err != nil {
			log.Printf("Input Port is not a valid 64 bit number\n")
		}
		log.Printf("Destination Host : %s\nDestination Port : %d\n", destHost, destPort)
	}

	log.Printf("KeyFileName : %s\n", *keyFileName)
	passphrase, errReadKeyFile := os.ReadFile(*keyFileName)
	if errReadKeyFile != nil {
		log.Fatalf("Error Reading passphrase from keyfile : %v", errReadKeyFile)
	}

	/*
		I am assuming that client is the one who always initates the connection
		and since this is a jump proxy this assumption is valid. Because of this
		assumption I need to generate the secure key inside the client first
		and then send the salt as plain text to the server as the first 16 bytes.
		The server can then generate the secure key on its side and then start
		decrypting as nonce, ciphertext
	*/

	if *listenPort == -1 {
		log.Printf("Client Mode\n")

		proxyConn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", destHost, destPort))
		if err != nil {
			log.Fatalf("Dial failed: %v", err)
		}
		defer proxyConn.Close()

		key, salt := generateKey(passphrase, nil)

		// write the salt as the first 16 bytes
		proxyConn.Write(salt)

		// Using WaitGroup to manage goroutines completion
		var wg sync.WaitGroup
		wg.Add(1)

		// Goroutine for copying server responses to stdout
		go func() {
			defer wg.Done()
			for {
				log.Printf("[Client] Awaiting Data From proxy")
				plaintext, errRecieveDecrypt := receiveDecrypted(proxyConn, key)
				log.Printf("[Client] Recieved from Proxy: %x", string(plaintext))
				if errRecieveDecrypt != nil || errRecieveDecrypt == io.EOF {
					log.Printf("[Client] errRecieveDecrypt : %v", errRecieveDecrypt)
					break
				}
				os.Stdout.Write(plaintext)
			}
		}()

		// Goroutine for reading from stdin and sending to the server
		go func() {
			defer wg.Done()
			reader := bufio.NewReader(os.Stdin)

			for {
				log.Printf("[Client] Awaiting Data From User/Process to Send to Proxy")
				plaintext := make([]byte, 1024)
				numBytesRead, errReadBytes := reader.Read(plaintext)
				if errReadBytes != nil {
					log.Printf("Client [ERROR] [Reading Input From Stdin] : %v", errReadBytes)
					break
				}
				plaintext = plaintext[0:numBytesRead]
				log.Printf("[Client] Sending to Proxy: %x", plaintext)
				sendEncrypted(proxyConn, plaintext, key)
				log.Printf("[Client] Sent to Proxy: %x", plaintext)
			}
		}()

		// Wait for both goroutines to finish
		wg.Wait()
	} else {
		listener, err := net.Listen("tcp", ":"+fmt.Sprintf("%d", *listenPort))
		if err != nil {
			log.Println("Error:", err)
			return
		}
		defer listener.Close()

		log.Printf("Server Mode: listening on port %d\n", *listenPort)

		for {
			// Accept incoming connections
			conn, err := listener.Accept()
			if err != nil {
				log.Println("Error:", err)
				continue
			}

			// Handle client connection in a goroutine
			go handleClient(conn, destHost, destPort, passphrase)
		}
	}
}

func safeClose(channel chan struct{}) {
	select {
	case <-channel:
		// Channel already closed
	default:
		close(channel)
	}
}

func handleClient(clientConn net.Conn, destHost string, destPort int64, passphrase []byte) {
	serviceConn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", destHost, destPort))
	if err != nil {
		log.Printf("Failed to connect to service: %v", err)
		clientConn.Close()
		return
	}
	salt := make([]byte, 16)
	_, errReadSalt := io.ReadFull(clientConn, salt)
	if errReadSalt != nil {
		log.Printf("[Proxy] Unable to read salt sent by client: %v", errReadSalt)
	}
	key, salt := generateKey(passphrase, salt)

	exitSignal := make(chan struct{})
	exitConfirm := make(chan struct{}, 2)

	// Accept data from client to proxy-service
	go func() {
		defer func() {
			clientConn.Close()
			serviceConn.Close()
			exitConfirm <- struct{}{} // Confirm this goroutine's exit
		}()
		for {
			select {
			case <-exitSignal: // Received exit signal from goroutine 2
				return
			default:
				log.Printf("[Proxy] Awaiting Data From Client\n")
				plaintext, errRecieveDecrypt := receiveDecrypted(clientConn, key)
				if errRecieveDecrypt == io.EOF {
					log.Printf("[Proxy] EOF Recieved breaking client reader")
					safeClose(exitSignal)
					break
				}
				if errRecieveDecrypt != nil {
					log.Printf("[Proxy] [ERROR] [Recieve Data from Client]: %v", errRecieveDecrypt)
					break
				}
				log.Printf("[Proxy] Recieved from Client: %x\n", plaintext)
				n, err := serviceConn.Write(plaintext)
				if err != nil {
					log.Printf("[Proxy] [ERROR] Proxy Client to Proxy: %v\n", err)
					break
				} else {
					log.Printf("Wrote %d bytes", n)
				}
				log.Printf("[Proxy] : Sent to service: %x\n", plaintext)
			}
		}
	}()

	// Send data from service to client
	go func() {
		defer func() {
			exitConfirm <- struct{}{} // Confirm this goroutine's exit
		}()

		reader := bufio.NewReader(serviceConn)
		for {
			select {
			case <-exitSignal: // Received exit signal from goroutine 1
				return
			default:
				log.Printf("[Proxy] Awaiting Data From Service\n")
				plaintext := make([]byte, 1024)
				numBytesRead, err := reader.Read(plaintext)
				if err == io.EOF {
					log.Printf("[Proxy] EOF Recieved Breaking Service Reader\n")
					safeClose(exitSignal)
					break
				}
				if err != nil {
					log.Printf("[Proxy] [ERROR] Proxy Error Reading Data From Service : %v\n", err)
					break
				}
				plaintext = plaintext[0:numBytesRead]
				log.Printf("[Proxy] Recieved from Service: %x\n", (plaintext))
				sendEncrypted(clientConn, plaintext, key)
				log.Printf("[Proxy] Sent to client: %x\n", (plaintext))
			}
		}
	}()

	<-exitConfirm
	<-exitConfirm
	log.Printf("Proxy : Closing Service and Client Connections\n")
}
