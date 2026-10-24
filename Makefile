PLATFORMS := darwin/arm64 darwin/amd64 linux/arm64 linux/amd64
PROTOCOL ?= ../protocol

test:
	go test ./...
	go -C protocol test ./...

# The binaries the plugin fetches where Go is not installed, for the release
# tagged v<version> of .claude-plugin/plugin.json.
release:
	rm -rf dist
	mkdir dist
	for p in $(PLATFORMS); do \
		GOOS=$${p%/*} GOARCH=$${p#*/} CGO_ENABLED=0 GOWORK=off \
			go build -trimpath -ldflags=-s -o dist/specrig-hook-$${p%/*}-$${p#*/} ./claude || exit 1; \
	done
	cd dist && shasum -a 256 specrig-hook-* >checksums.txt

protocol:
	rm -rf protocol/modes.yaml protocol/fixtures
	mkdir -p protocol/fixtures/guard
	cp $(PROTOCOL)/modes.yaml protocol/
	cp $(PROTOCOL)/fixtures/guard/*.yaml protocol/fixtures/guard/

.PHONY: test release protocol
