.PHONY: build release

build:
	mkdir -p build && go get && go build -o build/dockdash

release:
	mkdir -p build/releases && go get && for linux_arch in 386 amd64; \
		do \
		env GOOS=linux GOARCH=$$linux_arch go build -o build/releases/linux/$$linux_arch/dockdash && \
		(cd build/releases/linux/$$linux_arch/ && zip ../../dockdash_linux_$$linux_arch.zip  dockdash); \
		done;

