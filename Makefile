.PHONY: run build

run:
	go run ./cmd/godark

build:
	GOOS=darwin GOARCH=arm64 go build -ldflags "-X github.com/peter-stratton/dark-factory/internal/cmd.Version=$$(git describe --tags --always --dirty) -X github.com/peter-stratton/dark-factory/internal/cmd.BuildTime=$$(date -u '+%Y-%m-%dT%H:%M:%SZ')" -o bin/godark ./cmd/godark

install: build
	cp bin/godark ~/bin/godark
	codesign --force --sign - ~/bin/godark
	cp bin/godark ~/go/bin/godark
	codesign --force --sign - ~/go/bin/godark
