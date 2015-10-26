.PHONY: build release try

ROOT_DIR:=$(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))
REPO_PATH:=github.com/byrnedo/dockdash
GO_IMAGE:=golang:1.5.1

build:
	mkdir -p $(ROOT_DIR)/build && \
	docker run --rm -it\
		-v "$(ROOT_DIR)":/usr/src/$(REPO_PATH)\
		-w /usr/src/$(REPO_PATH) $(GO_IMAGE) \
		go get -d -v && go build -v -o build/dockdash 

#cross compile 386 and amd64
release:
	mkdir -p build/releases && \
	docker run --rm -it\
		-v "$(ROOT_DIR)":/usr/src/$(REPO_PATH) \
		-w /usr/src/$(REPO_PATH) $(GO_IMAGE) go get -d -v && bash do_release.sh

try:
	docker run --rm -it\
		-v /var/run/docker.sock:/var/run/docker.sock \
		-v "$(ROOT_DIR)":/usr/src/$(REPO_PATH) \
		-w /usr/src/$(REPO_PATH) $(GO_IMAGE) \
		go get -d -v && go build -v -o /tmp/dockdash && /tmp/dockdash

