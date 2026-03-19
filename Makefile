.PHONY: build test vet lint clean install

BINARY_DIR := bin
WYRD        := $(BINARY_DIR)/wyrd
MERGE_DRIVER := $(BINARY_DIR)/wyrd-merge-driver

build: $(WYRD) $(MERGE_DRIVER)

$(WYRD):
	@mkdir -p $(BINARY_DIR)
	go build -o $(WYRD) ./cmd/wyrd

$(MERGE_DRIVER):
	@mkdir -p $(BINARY_DIR)
	go build -o $(MERGE_DRIVER) ./cmd/wyrd-merge-driver

install:
	go install ./cmd/wyrd
	go install ./cmd/wyrd-merge-driver

test:
	go test ./... -v

vet:
	go vet ./...

lint:
	@which golangci-lint > /dev/null || (echo "golangci-lint not installed; run: brew install golangci-lint" && exit 1)
	golangci-lint run

clean:
	rm -rf $(BINARY_DIR)

# Run tests with coverage report
coverage:
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"
