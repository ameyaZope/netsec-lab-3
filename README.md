# JumpProxy in Golang

### TCP Communication over the Jump Proxy
You need to create three things with the below code

1. TCP Server
2. Jump proxy server
3. Jump proxy client
   
#### Creating TCP server via telnet

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
  nc -v -lk -p 9090
done

```

#### Jump Proxy Server Command
```bash
go run jumproxy.go -k test.txt -l 2222 localhost 9090
```

#### Jump Proxy Client Command
```bash
go run jumproxy.go -k test.txt <dstHost> <dstPort>
```

#### Creating SSH Jump Proxy Via Telnet
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

####
```bash
go run jumproxy.go -k mykey1 -l 2222 localhost 22
```
####
