package main

import (
	"bufio"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
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
		// salt = make([]byte, 16)
		salt = []byte("abcdef0123456789")
		if _, err := rand.Read(salt); err != nil {
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

	plaintext := make([]byte, len(ciphertext))
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
		log.Printf("[sendEncrypted] %v", errEncrypt);
		return errEncrypt
	}
	// Assuming a simple protocol: send the nonce size, nonce, and then ciphertext.
	nonceSizeBuf := make([]byte, 8)
	binary.BigEndian.PutUint64(nonceSizeBuf, uint64(len(nonce)))
	_, errWriteNonceSize := conn.Write(nonceSizeBuf)
	if errWriteNonceSize != nil {
		log.Printf("Client [ERROR] Writing NonceSize: %v\n", errWriteNonceSize)
		return errWriteNonceSize
	}
	_, errWriteNonce := conn.Write(nonce)
	if errWriteNonce != nil {
		log.Printf("Client [ERROR] Writing Nonce: %v\n", errWriteNonce)
		return errWriteNonce
	}

	cipherTextSize := len(ciphertext)
	sizeBuf := make([]byte, 8)
	binary.BigEndian.PutUint64(sizeBuf, uint64(cipherTextSize))
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
	nonceSizeBuf := make([]byte, 8)
	_, errReadNonceSize := conn.Read(nonceSizeBuf)
	nonceSize := int64(binary.BigEndian.Uint64(nonceSizeBuf))
	if errReadNonceSize != nil {
		log.Printf("[recieveDecrypt] Error Reading NonceSize : %v\n", errReadNonceSize)
		return nil, errReadNonceSize
	}

	nonce := make([]byte, nonceSize)
	_, errReadNonce := conn.Read(nonce)
	if errReadNonce != nil {
		log.Printf("[recieveDecrypt] Error Reading Nonce : %v\n", errReadNonce)
		return nil, errReadNonce
	}

	cipherTextSizeBuf := make([]byte, 8)
	_, errReadCipherTextSize := conn.Read(cipherTextSizeBuf)
	cipherTextSize := int64(binary.BigEndian.Uint64(cipherTextSizeBuf))
	if errReadCipherTextSize != nil {
		log.Printf("[recieveDecrypt] Error Reading CipherTextSize : %v\n", errReadCipherTextSize)
		return nil, errReadCipherTextSize
	}

	// Assuming the remaining data is the ciphertext. In practice, you might want to prefix the data with its size.
	ciphertext := make([]byte, cipherTextSize)
	n, errReadCipherText := conn.Read(ciphertext)
	if errReadCipherText != nil {
		log.Printf("[recieveDecrypt] Error Reading CipherText : %v\n", errReadCipherText)
		return nil, errReadCipherText
	}

	plaintext, errDecrpyt := decrypt(ciphertext[:n], key, nonce)
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
	passphrase, err := os.ReadFile(*keyFileName)

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

		conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", destHost, destPort))
		if err != nil {
			log.Fatalf("Dial failed: %v", err)
		}
		defer conn.Close()

		key, keyError := hex.DecodeString("186c82651c0d565e6d56541fc614036d33e857a143d44f915cca44e3687694ee")
		if keyError != nil {
			log.Printf("Proxy: Error Generating Key %v", keyError)
		}

		// write the salt as the first few bytes
		// conn.Write(salt)

		// Using WaitGroup to manage goroutines completion
		var wg sync.WaitGroup
		wg.Add(1)

		// Goroutine for copying server responses to stdout
		go func() {
			defer wg.Done()
			for {
				plaintext, errRecieveDecrypt := receiveDecrypted(conn, key)
				if errRecieveDecrypt != nil || errRecieveDecrypt == io.EOF {
					break
				}
				os.Stdout.Write(plaintext)
			}
		}()

		// Goroutine for reading from stdin and sending to the server
		go func() {
			defer wg.Done()
			for {
				reader := bufio.NewReader(os.Stdin)
				plaintext, errReadBytes := reader.ReadBytes('\n')
				if errReadBytes != nil {
					log.Printf("Client [ERROR] [Reading Input From Stdin] : %v", errReadBytes)
					break
				}
				sendEncrypted(conn, plaintext, key)
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
	key, keyError := hex.DecodeString("186c82651c0d565e6d56541fc614036d33e857a143d44f915cca44e3687694ee")
	if keyError != nil {
		log.Printf("Proxy: Error Generating Key %v", keyError)
		clientConn.Close()
		serviceConn.Close()
		return
	}

	exitSignal := make(chan struct{})
	exitConfirm := make(chan struct{}, 2)

	// Accept data from client to service
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
				log.Printf("Proxy : Awaiting Data From Client\n")
				plaintext, errRecieveDecrypt := receiveDecrypted(clientConn, key)
				if errRecieveDecrypt == io.EOF {
					log.Printf("EOF Recieved breaking client reader")
					safeClose(exitSignal)
					break
				}
				if errRecieveDecrypt != nil {
					log.Printf("Proxy [ERROR] [Recieve Data from Client]: %v", errRecieveDecrypt)
					break
				}
				log.Printf("Proxy : Recieved %s from service\n", plaintext)
				n, err := serviceConn.Write(plaintext)
				if err != nil {
					log.Printf("[ERROR] Proxy Client to Proxy: %v\n", err)
					break
				} else {
					log.Printf("Wrote %d bytes", n)
				}
				log.Printf("Proxy : Sent %s to service\n", plaintext)
			}
		}
	}()

	// Send data from service to client
	go func() {
		defer func() {
			exitConfirm <- struct{}{} // Confirm this goroutine's exit
		}()
		for {
			select {
			case <-exitSignal: // Received exit signal from goroutine 1
				return
			default:
				log.Printf("Proxy : Awaiting Data From Service\n")
				reader := bufio.NewReader(serviceConn)
				plaintext, err := reader.ReadBytes('\n')
				if err == io.EOF {
					log.Printf("EOF Recieved Breaking Service Reader\n")
					safeClose(exitSignal)
					break
				}
				if err != nil {
					log.Printf("[ERROR] Proxy Error Reading Data From Service : %v\n", err)
					break
				}
				log.Printf("Proxy : Recieved %s from Service\n", plaintext)
				sendEncrypted(clientConn, plaintext, key)
				log.Printf("Proxy : Sent %s to client in encrypted form\n", plaintext)
			}
		}
	}()

	<-exitConfirm
	<-exitConfirm
	log.Printf("Proxy : Closing Service and Client Connections\n")
}
