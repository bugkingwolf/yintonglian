.PHONY: run build test check clean
run:
	go run ./cmd/yintonglian
build:
	mkdir -p bin
	CGO_ENABLED=0 go build -trimpath -o bin/yintonglian ./cmd/yintonglian
test:
	go test ./... -count=1
check:
	go vet ./...
	go test -race ./... -count=1
clean:
	rm -f bin/yintonglian
