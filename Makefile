.PHONY: build run test vet parity loadtest
build:
	mkdir -p bin && go build -trimpath -ldflags="-s -w" -o bin/server ./cmd/server && go build -o bin/loadtest ./cmd/loadtest
run:            ## in-memory store unless DATABASE_URL is set
	go run ./cmd/server
test:           ## set TEST_DATABASE_URL to an empty scratch database to include the PostgreSQL tests
	go test -race -count=1 ./...
vet:
	go vet ./...
parity:         ## client physics must match the server physics
	go run ./tools/parity > /tmp/parity.json && node tools/parity/check.mjs /tmp/parity.json && node tools/parity/clientsmoke.mjs /tmp/parity.json
loadtest:       ## start the server with DISABLE_RATE_LIMITS=1 PBKDF2_ITER=1000 first
	go run ./cmd/loadtest -n 500 -d 30s
