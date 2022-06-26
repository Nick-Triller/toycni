BINARY_NAME=toycni

.PHONY: build
build:
	GOOS=linux GOARCH=amd64 go build -o bin/${BINARY_NAME}

.PHONY: demo-setup
demo-setup: build
	./demo/setup.sh

.PHONY: demo-cleanup
demo-cleanup:
	./demo/cleanup.sh
