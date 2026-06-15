GOOS   := $(shell go env GOOS)
GOARCH := $(shell go env GOARCH)

BINARIES := gi-get-stats ww-get-stats

.PHONY: all clean dist

all: $(BINARIES)

gi-get-stats:
	go build -ldflags="-s -w" -trimpath -o $@ ./cmd/gi-get-stats

ww-get-stats:
	go build -ldflags="-s -w" -trimpath -o $@ ./cmd/ww-get-stats

clean:
	rm -f $(BINARIES)
	rm -rf dist/

dist: $(BINARIES)
	mkdir -p dist
	tar -czf dist/gacha-stats-$(GOOS)-$(GOARCH).tar.gz $(BINARIES)
