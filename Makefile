.PHONY: deps sync-web build run test vet loadtest
deps:
	go mod download
sync-web:
	rm -rf cmd/server/web
	cp -R web cmd/server/web
build: deps sync-web
	mkdir -p bin
	go build -trimpath -ldflags="-s -w" -o bin/ditu ./cmd/server
run: deps sync-web
	go run ./cmd/server
test: deps sync-web
	go test ./...
vet: deps sync-web
	go vet ./...
loadtest: deps
	go run ./cmd/loadtest -n 2000 -rate 1
