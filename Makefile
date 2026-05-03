.PHONY: build test vet lint lint-tui clean install demo screenshots

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

lint: lint-tui
	@which golangci-lint > /dev/null || (echo "golangci-lint not installed; run: brew install golangci-lint" && exit 1)
	golangci-lint run

# Catches bare " " / "  " used as separators directly between Render() calls
# in TUI code. Matches: .Render(...) + " " + and .Render(...) + "  " +
# Use Spacer(n, bg) instead. See CLAUDE.md TUI styling rules.
lint-tui:
	@echo "Checking for bare spacers between Render() calls in TUI code..."
	@! grep -rn --include='*.go' -E '\.Render\([^)]*\)[[:space:]]*\+[[:space:]]+"[[:space:]]+"[[:space:]]*\+' internal/tui/ \
		| grep -v '_test.go' \
		|| (echo "FAIL: bare spacers found between Render() calls — use Spacer(n, bg) instead (see CLAUDE.md TUI styling rules)" && exit 1)
	@echo "OK"

clean:
	rm -rf $(BINARY_DIR)

# Run tests with coverage report
coverage:
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

demo:
	@echo "Generating VHS demo recordings..."
	@which vhs > /dev/null || (echo "vhs not installed; run: go install github.com/charmbracelet/vhs@latest" && exit 1)
	@echo "No tape files yet — add .tape files to docs/vhs/ and update this target."

screenshots:
	@echo "Generating screenshots with freeze..."
	@which freeze > /dev/null || (echo "freeze not installed; run: go install github.com/charmbracelet/freeze@latest" && exit 1)
	@echo "No screenshot scripts yet — add scripts to docs/screenshots/ and update this target."
