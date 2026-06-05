BINARY := ai-mux
BUILD_DIR := bin
VERSION ?= dev
LDFLAGS := -ldflags "-X github.com/creydr/ai-mux/cmd/ai-mux/commands.Version=$(VERSION)"

.PHONY: build test lint fmt clean coverage run-daemon run-dashboard

build:
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY) ./cmd/ai-mux

test:
	go test ./... -race -count=1

lint:
	@echo "==> gofmt"
	@test -z "$$(gofmt -l .)" || (gofmt -l . && exit 1)
	@echo "==> go vet"
	go vet ./...

fmt:
	gofmt -w .
	goimports -w .

clean:
	rm -rf $(BUILD_DIR)

coverage:
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out

run-daemon:
	go run ./cmd/ai-mux daemon start

run-dashboard:
	go run ./cmd/ai-mux dashboard

integration-test:
	go test ./... -tags=integration -race -count=1
