.PHONY: build release docker docker-run

ROOT_DIR:=$(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))

build:
	mkdir -p $(ROOT_DIR)/build && \
	docker run --rm -it\
		-v "$(ROOT_DIR)":/usr/src/github.com/byrnedo/dockdash \
		-w /usr/src/github.com/byrnedo/dockdash golang:1.5.1 \
		go get -d -v && go build -v -o build/dockdash 

#cross compile 386 and amd64
release:
	mkdir -p build/releases && \
	docker run --rm -it\
		-v "$(ROOT_DIR)":/usr/src/github.com/byrnedo/dockdash \
		-w /usr/src/github.com/byrnedo/dockdash golang:1.5.1 go get -d -v && bash -c \
		"for linux_arch in 386 amd64; \
		do \
		env GOOS=linux GOARCH=\$$linux_arch go build -o build/releases/linux/\$$linux_arch/dockdash && \
		(cd build/releases/linux/\$$linux_arch/ && zip ../../dockdash_linux_\$$linux_arch.zip  dockdash); \
		done;"

try:
	docker run --rm -it\
		-v /var/run/docker.sock:/var/run/docker.sock \
		-v "$(ROOT_DIR)":/usr/src/github.com/byrnedo/dockdash \
		-w /usr/src/github.com/byrnedo/dockdash golang:1.5.1 \
		go get -d -v && go build -v -o /tmp/dockdash && /tmp/dockdash

