.PHONY: run build

run:
	go run ./cmd/godark

build:
	GOOS=darwin GOARCH=arm64 go build -ldflags "-X github.com/phs/dark-factory/internal/cmd.BuildTime=$$(date -u '+%Y-%m-%dT%H:%M:%SZ')" -o bin/godark ./cmd/godark

install: build
	cp bin/godark ~/bin/godark
