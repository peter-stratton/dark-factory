.PHONY: run build

run:
	go run ./cmd/godark

build:
	GOOS=darwin GOARCH=arm64 go build -o bin/godark ./cmd/godark
