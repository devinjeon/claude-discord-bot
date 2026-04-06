BINARY := claude-bot
BUILD_DIR := ./cmd/claude-bot
COVERAGE_DIR := ./coverage
COVERAGE_FILE := $(COVERAGE_DIR)/coverage.out

.PHONY: build run clean install add-channel uninstall test test-verbose coverage coverage-html vet lint check

build:
	go build -o $(BINARY) $(BUILD_DIR)

run: build
	./$(BINARY)

clean:
	rm -f $(BINARY)
	rm -rf $(COVERAGE_DIR)

install:
	bash deploy/install.sh

add-channel:
	bash deploy/install.sh --no-build

uninstall:
	bash deploy/uninstall.sh

test:
	go test ./...

test-verbose:
	go test -v ./...

test-race:
	go test -race ./...

coverage:
	@mkdir -p $(COVERAGE_DIR)
	go test -coverprofile=$(COVERAGE_FILE) -covermode=atomic ./...
	go tool cover -func=$(COVERAGE_FILE)

coverage-html: coverage
	go tool cover -html=$(COVERAGE_FILE) -o $(COVERAGE_DIR)/coverage.html
	@echo "Coverage report: $(COVERAGE_DIR)/coverage.html"

vet:
	go vet ./...

check: vet test-race
