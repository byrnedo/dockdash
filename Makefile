.PHONY: build release d-build try version

ROOT_DIR:=$(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))
REPO_PATH:=github.com/byrnedo/dockdash
GO_IMAGE:=golang:1.18

build:
	go build -v -o build/dockdash


release:
	goreleaser --rm-dist

d-build:
	mkdir -p $(ROOT_DIR)/build && \
	docker run --rm -it\
		-v "$(ROOT_DIR)":/usr/src/$(REPO_PATH)\
		-w /usr/src/$(REPO_PATH) $(GO_IMAGE)

try:
	docker run --rm -it\
		-v /var/run/docker.sock:/var/run/docker.sock \
		-v "$(ROOT_DIR)":/usr/src/$(REPO_PATH) \
		-w /usr/src/$(REPO_PATH) $(GO_IMAGE) \
		go get -d -v && go build -v -o /tmp/dockdash && /tmp/dockdash

version:
	echo "package main\nconst VERSION=\"$$(git describe --tags)\"" > version.go

