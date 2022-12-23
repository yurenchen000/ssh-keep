ssh-keep
========

keep your ssh connection survive from network fluctuation or wifi switching

Inspired by
https://eternalterminal.dev

## how to use

### build

```sh
GO111MODULE=off GOPATH=$PWD go build -o ssh-keep-c client.go
GO111MODULE=off GOPATH=$PWD go build -o ssh-keep-s server.go
```

then got
- ssh-keep-c //the client side, ssh proxy cmd
- ssh-keep-s //the server side, ssh relay


### deploy


#### server side

listen on a tcp port (:2021 for example, for client connect), connect to your ssh server :22.
```
./ssh-keep-s -server 127.0.0.1:22 -listen :2021
```

#### client side

```
ssh -o ProxyCommand='ssh-keep-c --server %h:2021 2>/dev/null' your_ssh_server
```

or put it into ssh_config

```conf
Host your_ssh_server
    ProxyCommand ssh-proxy --server %h:2021 2>/dev/null
```

