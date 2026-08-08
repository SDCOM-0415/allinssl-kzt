BINARY := hydun-cdn
DIST := dist

.PHONY: build build-linux-amd64 build-linux-arm64 clean test

build:
	mkdir -p $(DIST)
	go build -trimpath -o $(DIST)/$(BINARY) .

build-linux-amd64:
	mkdir -p $(DIST)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o $(DIST)/$(BINARY)-linux-amd64 .

build-linux-arm64:
	mkdir -p $(DIST)
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags="-s -w" -o $(DIST)/$(BINARY)-linux-arm64 .

clean:
	rm -rf $(DIST)

test:
	printf '%s' '{"action":"get_metadata"}' | go run .
