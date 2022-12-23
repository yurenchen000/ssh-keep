ssh-keep
========

## 🍵 what's this

keep your ssh connection survive from network fluctuation or wifi switching

Inspired by
https://eternalterminal.dev


It work as a relay-connection between ssh-client and ssh-server.


```
                                        [ssh server]
                                             |
                                            tcp
                                             |
relay-client ---- relay-connection ---- relay-server
    |
  stdio
    |
[ssh client]
```

the relay-client & relay-server fake a persistent connection,  
quietly reconnect and never noitfy ssh-client/ssh-server.


## 🍵 build

```bash
GO111MODULE=off GOPATH=$PWD go build -o ssh-keep-c client.go
GO111MODULE=off GOPATH=$PWD go build -o ssh-keep-s server.go
```

then got
- ssh-keep-c //the client side, ssh proxy cmd
- ssh-keep-s //the server side, ssh relay


## 🍵 deploy


### 1. server side

connect to your real ssh server :22.
and listen on a tcp port (:2021 for example, wait for client connect)

```
./ssh-keep-s -server 127.0.0.1:22 -listen :2021
```

### 2. client side

```bash
ssh -o ProxyCommand='ssh-keep-c --server %h:2021 2>/dev/null' your_ssh_server
```

or put it into ssh_config

```conf
## setup a ssh-keep client
Host your_ssh_server
    ProxyCommand ssh-keep-c --server %h:2021 2>/dev/null
```

//then it can also work as a jump host (get the benefit of stable connection)
```
## other can use it as a jump host
Host other_ssh_server
    ProxyJump your_ssh_server
```


## tips
when lose connect with server, you can't exit by `Ctrl+D`  
//that's bash exit key, but you lose connection with it.

use 
- `Enter ~ . ` to exit 
- `Enter ~ ? ` for help
