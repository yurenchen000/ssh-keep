
# get version from git tag
#   0.9.5 
#   0.9.5-dirty 
#   0.9.4-4-7f51ff0-dirty
define GIT_VER
  VN=`git describe --tags --match "v[0-9]*" HEAD 2>/dev/null`;
  VN=`echo "$$VN" | sed -e 's/^v//' -e 's/-g/-/'`;
  git update-index -q --refresh;
  test -z "`git diff-index --name-only HEAD --`" || VN="$$VN-dirty";
  echo $$VN
endef

VER=$(shell ${GIT_VER})

FLAGS= -ldflags='-s -w -X main.build_version=${VER}' 

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

test:
	@echo " VER: $(VER)"
	@echo "DEST: $(DEST)"

clean:
	rm -f ssh-keep-c ssh-keep-s
