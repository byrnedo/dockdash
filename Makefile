.PHONY: build release

build:
	mkdir -p build && go get && go build -o build/dockdash

release:
	mkdir -p build/releases && \
		go get && \
		env GOOS=linux GOARCH=386 go build -o build/releases/linux/386/dockdash &&\
		(cd build/releases/linux/386/ && zip ../../dockdash_linux_386.zip  dockdash) &&\
		env GOOS=linux GOARCH=amd64 go build -o build/releases/linux/amd64/dockdash &&\
		(cd build/releases/linux/amd64/ && zip ../../dockdash_linux_amd64.zip dockdash)
