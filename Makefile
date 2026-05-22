BINARY := claude-bot
BUILD_DIR := ./cmd/claude-bot
COVERAGE_DIR := ./coverage
COVERAGE_FILE := $(COVERAGE_DIR)/coverage.out

.PHONY: build run clean install add-channel remove-channel restart-channels uninstall test test-verbose coverage coverage-html vet lint check

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
	@test -n "$(NAME)" -a -n "$(CHANNEL)" -a -n "$(DIR)" || { echo "Usage: make add-channel NAME=<name> CHANNEL=<id> DIR=<path>"; exit 1; }
	bash deploy/manage-channel.sh add "$(NAME)" "$(CHANNEL)" "$(DIR)"

remove-channel:
	@test -n "$(NAME)" || { echo "Usage: make remove-channel NAME=<name>"; exit 1; }
	bash deploy/manage-channel.sh remove "$(NAME)"

restart-channels:
	bash deploy/restart-channels.sh $(ARGS)

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
