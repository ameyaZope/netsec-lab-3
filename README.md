# JumpProxy in Golang

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
