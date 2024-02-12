BINARY_NAME:=testwave
IMAGE_NAME:=$(BINARY_NAME)-image

build:
	go build -o bin/$(BINARY_NAME) -v .

clean:
	rm -rf bin/

start: build
	./bin/$(BINARY_NAME) start
	./bin/$(BINARY_NAME) logs

logs:
	./bin/$(BINARY_NAME) logs
	
.PHONY: build clean start