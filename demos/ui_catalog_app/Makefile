.PHONY: build test clean run

BINARY=ui_catalog

build:
	go build -o $(BINARY) .

test:
	go test ./... -v

clean:
	rm -f $(BINARY)

run: build
	./$(BINARY)

check:
	go vet ./...
	go fmt ./...
