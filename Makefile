APP_DIR ?= $(HOME)/Applications
BIN_DIR ?= $(HOME)/.local/bin
VERSION ?= 0.1.0

.PHONY: all build test check install package clean

all: build

build:
	@VERSION="$(VERSION)" ./scripts/build.sh

test:
	CGO_CFLAGS="$(CGO_CFLAGS) -mmacosx-version-min=13.0" CGO_LDFLAGS="$(CGO_LDFLAGS) -mmacosx-version-min=13.0" go test ./...

check:
	@mkdir -p build
	CGO_CFLAGS="$(CGO_CFLAGS) -mmacosx-version-min=13.0" CGO_LDFLAGS="$(CGO_LDFLAGS) -mmacosx-version-min=13.0" go build -trimpath -ldflags="-s -w" -o build/kol-check ./cmd/kol-check
	./build/kol-check -stress

install: build
	@APP_DIR="$(APP_DIR)" BIN_DIR="$(BIN_DIR)" ./scripts/install.sh

package: build
	@VERSION="$(VERSION)" ./scripts/package.sh

clean:
	rm -rf build dist
