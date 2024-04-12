# JumpProxy in Golang

## Brief Description of Program

This Go program implements an encrypted network proxy that can operate in both client and server modes. It utilizes AES-GCM for encryption to ensure confidentiality and integrity of the data transmitted over the network. Key derivation is performed using PBKDF2 with a SHA-256 hash function, ensuring secure key generation from a passphrase.

The program defines constants for cryptographic parameters like salt, nonce, and key lengths, as well as a block size for plaintext data handling. Key functions include generateKey for deriving a secure key from a passphrase and salt, encrypt and decrypt for handling data encryption and decryption respectively, and sendEncrypted and receiveDecrypted for sending and receiving encrypted data over a network connection.

The main execution starts by parsing command line arguments to determine the operation mode (client or server) and necessary parameters such as key file and network ports. In client mode, the program establishes a TCP connection to a specified destination, generates an encryption key, and handles data transmission and reception using separate goroutines to maintain asynchronous input/output. In server mode, it listens for incoming connections and relays decrypted client data to a specified destination server.

Error handling is robust, with checks at every step where an operation might fail, including file and network operations. The use of logging is extensive, providing detailed error messages and operational status, which are directed to specific log files depending on the mode of operation. 

## Usage 

### End to End SSH using Jumproxy.go
1. Ensure that your ssh server is running on port 22
2. Run the following command to start the server side jumproxy on the same machine as your ssh server. Please note that the test.txt file contains the passphrase. The below command runs the jumproxy in reverse proxy mode
```bash
  go run jumproxy.go -k test.txt -l 2222 localhost 22
```
3. Run the below command to compile the jumproxy.go file into an executable
```bash
  go build jumproxy.go
```
4. On the client machine from which you want to initiate an ssh connection, run the below command. The below command runs the jumproxy in client mode. 
```bash
ssh -o "ProxyCommand <Absolute Path To jumproxy executable> -k <Absolute Path to file containing passphrase> <IP Address of Host on Which SSH Server is running> 2222" <UserName for authentication>@localhost -vvv
```

### End to End Encrypted TCP Communication over the Jump Proxy
1. Run a plain TCP server using either ***nc*** or ***ncat***. Use any one of the two below commands, I would prefer the ncat command, the reason for the same is given at the end of the README.md
```bash
ncat -lkv 9090 
nc -lkv -p 9090
```
2. Run the jumproxy server using the below command
```bash
go run jumproxy.go -k test.txt -l 2222 localhost 9090
```
3. Run the jumproxy client using the below command
```bash
go run jumproxy.go -k test.txt <IP_Address_Of_Machine_On_Which_Jumproxy_Server_Is_Running> 2222
```
The above would open a fully encrypted, fully duplex connection to the ncat server. This is like a two way chat application. My application supports multiple clients running together. Whenever multiple clients are running together and you type something on the ncat server console, it is broadcasted to all clients.    

## Need for Jumproxy
The jump proxy that I have developed, named 'jumproxy', adds an extra layer
of encryption to connections towards TCP services. Instead of connecting
directly to the service, clients connect to jumproxy (running on the same
server that hosts the service), which then relays all traffic to the actual
service. Before relaying the traffic, jumproxy always decrypts it using a
static symmetric key. This means that if the data of any connection towards
the protected server is not properly encrypted, then the server will terminate
the connection.

This is a better option than changing the port number of the SSH server, port
knocking,  and other similar security-by-obscurity solutions, as attackers who
might want to exploit a zero day vulnerability in the protected service would
first have to know the secret key for having a chance to successfully deliver
their attack vector to the server. This of course assumes that the jump proxy
does not suffer from any vulnerability itself. Given that its task and its
code are much simpler compared to an actual service (e.g., an SSH server), and
thus its code can be audited more easily, it can be more confidently exposed
as a publicly accessible service. Furthermore, Go is a memory-safe language
that does not suffer from memory corruption bugs.

## Jumproxy Encryption

### Key Establishment
The usage of jumproxy assumes that the passphrase is securely made available on both the client and the target machine. The passphrase is then used to derive a key based on a nonce that is salt chosen by the client. This salt is then passed to the jumproxy-server in plaintext as the first few bytes of the communication. The jumproxy-server then uses the salt with the passphrase to derive the same key that the client had generated. 

### Subsequent Communication
Once the key has been established the subsequent communication starts. In this communication we use AES-256 in GCM mode to encrypt the communication. 


## Creation of Application Layer Protocol
This project is a classical example of how to create and then implement an application layer protocol given that you have chosen the underlying transport layer protocol. The underlying transport layer protocol here is tcp and we can call the application layer protocol as a secure proxy. Now we will go through the jumproxy protocol that we have implemented. 

### Protocol

#### Key Generation from passphrase and handling Salt
The key is generated using pbkdf2 package. We provide the passphrase and a randomly generated salt to the pbkdf2 package and it generates the key for us. Client is the first one to generate the key and hence, it is the one who generates the salt as well. The salt is a SALT_LENGTH byte sequence that is sent in plaintext as the first 16 bytes of the tcp communication. As soon as the client generates the salt, it writes the first 16 bytes of the communication as the salt. The server on the other hand, interprets the first 16 bytes of the tcp communication as the salt and uses this salt along with the pre set passphrase to generate the key. 

#### Client to Proxy-Client
Below we define the reading strategy at client side and then in the third step we define the strategy to send packets to Proxy-Client
1. Read 1024 bytes max from the input(stdin). Get the number of bytes read as numBytesRead
2. ciphertext = encrypt(plaintext, key)
3. Send nonce, cipherTextSize, ciphertext to proxy-client(To send to proxy-client just write to stdout). Here we assume that nonce is always of size 12 bytes. For sending cipherTextSize there are two ways
   1. Send the number cipherTextSize as a sequence of bytes directly converted from string. A positive number less than 65,536 will take ,minimum one byte per character to transfer over the wire (assuming UTF-8 encoding)
   2. Convert the number cipherTextSize into the uint16 BigEndian representation of the number and then send this BigEndian representation. Here a positive number less than 65,536 will take 2 bytes to send. 
The second approach is better here because because it sends the number in somewhat of a compressed format. 


#### Proxy-Client to Client
Here we define the startegy to read network data coming from proxy-service recieved by proxy-client
1. Read 12 byte nonce
2. Read 2 byte cipherTextSize --> 16 bytes uint BigEndian Format
3. Read ciphertext of ciphertextSize
4. plaintext = decrypt(ciphertext, nonce, key)
5. Send plaintext to service


#### Proxy-Service to Service communication
Here we define the startegy to read network data coming from proxy-client recieved by proxy-service
1. Read 12 byte nonce
2. Read 2 byte cipherTextSize --> 16 bytes uint BigEndian Format
3. Read ciphertext of ciphertextSize
4. plaintext = decrypt(ciphertext, nonce, key)
5. Send plaintext to service



#### Service to Proxy-service communication
1. Read 1024 bytes max from the input(stdin). Get the number of bytes read as numBytesRead
2. ciphertext = encrypt(plaintext, key)
3. Send nonce, cipherTextSize, ciphertext to proxy-service(To send to proxy-client just write to stdout). Here we assume that nonce is always of size 12 bytes. For sending cipherTextSize there are two ways
   1. Send the number cipherTextSize as a sequence of bytes directly converted from string. A positive number less than 65,536 will take ,minimum one byte per character to transfer over the wire (assuming UTF-8 encoding)
   2. Convert the number cipherTextSize into the uint16 BigEndian representation of the number and then send this BigEndian representation. Here a positive number less than 65,536 will take 2 bytes to send. 
The second approach is better here because because it sends the number in somewhat of a compressed format.


#### Debugging Experience
**Problem:** One major problem that I hit during implementation is that first I got a plain tcp connection to start working. I was taking bytes input from stdin from client side and I was recieveing bytes on proxy-service and relaying those bytes to my tcp service hosted by ncat command. The ncat command would print those bytes slice as a string on the terminal. I confirmed that this functionality was working, then I went ahead to test if my jumproxy correctly handles ssh communication. Here I encountered the problem that my client recieved the banner from the ssh server. Then my client sent the next byte sequence to the ssh server and immediately recieved the string "Invalid SSH identification string.". This was happening because whenever my client was sending bytes, it was sending over extra bytes. More specifically it was sending "U+0000 <control> character" which is the NUL character in. On the proxy I tried debugging this by printing the byte sequence as a string. The NUL character does not print anything when this happens but the ssh server detects this NUL character and sends that the SSH ideentification string is invalid. 

**Detection** To detect this NUL character I print out the bytes sequence as a hex string. Here the NUL characters was clearly visible. I dug deep into why the NUL characters were present, it was because I was predefining a large slice as the plaintext and the acm.Open function used to append data to the plaintext slice, which meant that my plaintext byte slice would be prepended with many NUL characters. I removed the predefining of the size and now there were no NUL characters and the ssh protocol worked perfectly. 

## Extras

### Creating SSH Jump Proxy Via NetCat

#### Running a script that forwards connections to ssh server
```bash
#!/bin/bash

cleanup() {
  echo "Terminating..."
  exit
}

# Trap SIGINT (Ctrl+C) signal and execute the 'cleanup' function
trap cleanup SIGINT

while true
do
  nc -v -lk -p 9090 -c 'nc localhost 22'
done

```

#### SSH Into Remote via Jump proxy

```bash
ssh -J vboxuser@172.24.24.100:2222 vboxuser@localhost
```

### Need for Replacement for nc
Please note that the problem with a TCP servier using nc is that it inherently does not handle multiple tcp connections. The second TCP connection will be rejected (assuming that there is already one ongoing TCP connection). The solution for this is to use **socat** command. The problem with socat is that it is not inherently built for a two way communication that I am looking for. A better solution is to use **ncat**. ncat is the modern version of netcat which can handle multiple connections, ssl and many other modern features. 

```bash
ncat -lkv 9090
```

### Generating large file
I have created two programs to create large files with alphanumerica characters, the cpp one works way faster, just use it. Below are the copmmands that work for me on MacOS

For running cpp code
```bash
g++-12 -o run create_large_file.cpp
./run
```

For running python code
```bash
python3 create_large_file.py
```
