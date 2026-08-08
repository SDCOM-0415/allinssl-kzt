BINARY := hydun-cdn
DIST := dist
VERSION ?= dev

LDFLAGS_BASE := -X main.Version=$(VERSION)
LDFLAGS_RELEASE := -s -w $(LDFLAGS_BASE)

# 默认构建当前平台 release 版本。
.PHONY: build build-debug test test-debug clean

build:
	mkdir -p $(DIST)
	go build -trimpath -ldflags="$(LDFLAGS_RELEASE)" -o $(DIST)/$(BINARY) .

build-debug:
	mkdir -p $(DIST)
	go build -trimpath -tags debug -ldflags="$(LDFLAGS_BASE)" -o $(DIST)/$(BINARY)-debug .

test:
	printf '%s' '{"action":"get_metadata"}' | go run -ldflags="$(LDFLAGS_RELEASE)" .

test-debug:
	printf '%s' '{"action":"get_metadata"}' | go run -tags debug -ldflags="$(LDFLAGS_BASE)" .

clean:
	rm -rf $(DIST)
