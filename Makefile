.PHONY: build test clean lint run

BINARY = mu-bmd-renderer
GO = /usr/local/go/bin/go

build:
	$(GO) build -o $(BINARY) ./cmd/render

test:
	$(GO) test ./internal/...

lint:
	$(GO) vet ./...

clean:
	rm -f $(BINARY)

run: build
	./$(BINARY)

# Quick test: render 5 items
test-quick: build
	./$(BINARY) -test 5

# Render a single item (e.g., Katana: section 0, index 3)
test-single: build
	./$(BINARY) -section 0 -index 3

tidy:
	$(GO) mod tidy

deps:
	$(GO) mod download
