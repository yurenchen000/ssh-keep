ssh-keep
========

[![build](https://github.com/yurenchen000/ssh-keep/actions/workflows/release.yml/badge.svg)](https://github.com/yurenchen000/ssh-keep/releases)
[![go-report](https://goreportcard.com/badge/github.com/yurenchen000/ssh-keep)](https://goreportcard.com/report/github.com/yurenchen000/ssh-keep)

[![release](https://img.shields.io/github/v/release/yurenchen000/ssh-keep)](https://github.com/yurenchen000/ssh-keep/releases)

<br>

## 🧐 what's this

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

<br>

## 🛠️ build
// you don't have to build it, pre-build binary can be download at release page.
```console
GO111MODULE=off GOPATH=$PWD go build -o ssh-keep-c client.go
GO111MODULE=off GOPATH=$PWD go build -o ssh-keep-s server.go
```

then got
- ssh-keep-c //the client side, ssh proxy cmd
- ssh-keep-s //the server side, ssh relay

<br>

## 🚀 deploy


### 1. server side

connect to your real ssh server :22.
and listen on a tcp port (:2021 for example, wait for client connect)

Run manually
```console
./ssh-keep-s -server 127.0.0.1:22 -listen :2021
```

OR Use systemd service:
```bash
#install
sudo cp -pvi ssh-keep-s       /usr/local/bin/
sudo cp -pvi ssh-keep.service /etc/systemd/system/
sudo systemctl daemon-reload

#start
sudo systemctl start ssh-keep.service

#auto start
sudo systemctl enable ssh-keep.service
```

<br>

**sshd_config**  
// or put it into `/etc/ssh/sshd_config.d/ssh_keep.conf`
```sshdconfig
############# local conn for ssh-keep
Match Address 127.0.0.1,::1
    # max 20 day timeout
    ClientAliveInterval 3600
    ClientAliveCountMax 480
```

// reload sshd_config  
`sudo systemctl reload ssh`

<br>

### 2. client side

```console
ssh -o ProxyCommand='ssh-keep-c --server %h:2021 2>/dev/null' your_ssh_server
```

or put it into **ssh_config**

```sshconfig
## setup a ssh-keep client
Host your_ssh_server
    ProxyCommand ssh-keep-c --server %h:2021 2>/dev/null

    ## not send alive msg (Useful when PC suspend/hibernate/offline hours)
    ServerAliveInterval 0
    TCPKeepAlive no

    ## or long timeout:  (max 20 day timeout)
    #ClientAliveInterval 3600
    #ClientAliveCountMax 480
```

// then it can also work as a jump host (get the benefit of stable connection)
```sshconfig
## other can use it as a jump host
Host other_ssh_server
    ProxyJump your_ssh_server
```

<br>

## 💡 tips
when lose connect with server, you can't exit by `Ctrl+D`  
// that's bash exit key, but you lose connection with it.

use 
- `Enter ~ . ` to exit 
- `Enter ~ ? ` for help

// that's ssh special key.


<br>

## 🧰 Related Tools

[![related-repos](https://res.ez2.fun/svg/repos-ssh_enhance.svg)](https://github.com/yurenchen000/yurenchen000/blob/main/repos.md#ssh-enhance)


