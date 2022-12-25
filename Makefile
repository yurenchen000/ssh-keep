
FLAGS= -ldflags='-s -w'

all: ssh-keep-c ssh-keep-s
	echo "DEST: $(DEST)"
	[ -z "$(DEST)" ] || cp -pv $^ $(DEST)

ssh-keep-c: client.go
	GO111MODULE=off GOPATH=$(PWD) go build -o $@ $(FLAGS) $^

ssh-keep-s: server.go
	GO111MODULE=off GOPATH=$(PWD) go build -o $@ $(FLAGS) $^

#hello: hello.go
#	GO111MODULE=off go build -o $@ $(FLAGS)
#	echo "^: $^"
#	echo "@: $@"

clean:
	rm -f ssh-keep-c ssh-keep-s
