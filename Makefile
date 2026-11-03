VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: build test lint dev clean

build:
	go build -ldflags "$(LDFLAGS)" -o dist/alis ./cmd/alis

test:
	go test ./...

lint:
	go vet ./...
	shellcheck -x lib/alis.sh packs/*/bin/*

# Try this checkout in your current shell: eval "$(make -s dev)"
dev: build
	@echo 'export ALIS_SHARE_DIR="$(CURDIR)" PATH="$(CURDIR)/dist:$$PATH"; eval "$$(alis init zsh)"'

clean:
	rm -rf dist
