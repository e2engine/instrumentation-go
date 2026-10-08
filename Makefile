ROOT_PATH := $(dir $(realpath $(lastword $(MAKEFILE_LIST))))
COVERAGE_PATH := $(ROOT_PATH).coverage/
BIN_PATH ?= $(ROOT_PATH)bin

GO_VERSION ?= 1.25.0
GO_TOOLCHAIN ?= go$(GO_VERSION)

MODULES := . http grpc aws

# Ensure our local tools are preferred
export PATH := $(ROOT_PATH)tools/bin:$(PATH)

include $(ROOT_PATH)tools/tools.mk

.PHONY: tidy
tidy:
	@echo "Running go mod tidy..."
	@for module in $(MODULES); do \
		echo "Tidying $$module..."; \
		(cd $$module && \
			GOTOOLCHAIN=$(GO_TOOLCHAIN) go mod tidy) || exit $$?; \
	done

.PHONY: lint
lint: install-golangci-lint
	@echo "Running Go linter..."
	@for module in $(MODULES); do \
		echo "Linting $$module..."; \
		(cd $$module && $(GOLANGCI_LINT) run) || exit $$?; \
	done

.PHONY: lint-fix
lint-fix: install-golangci-lint
	@echo "Running Go linter with fixes..."
	@for module in $(MODULES); do \
		echo "Linting $$module..."; \
		(cd $$module && $(GOLANGCI_LINT) run --fix) || exit $$?; \
	done

.PHONY: test
test:
	@echo "Running tests..."
	@for module in $(MODULES); do \
		echo "Testing $$module..."; \
		(cd $$module && \
			GOTOOLCHAIN=$(GO_TOOLCHAIN) go clean -testcache && \
			GOTOOLCHAIN=$(GO_TOOLCHAIN) go test ./... -count=1 -timeout=600s) || exit $$?; \
	done

.PHONY: test-cov
test-cov:
	@echo "Running tests with coverage..."
	@rm -rf $(COVERAGE_PATH)
	@mkdir -p $(COVERAGE_PATH)
	@for module in $(MODULES); do \
		name=$$(basename "$$module"); \
		if [ "$$module" = "." ]; then name=root; fi; \
		echo "Testing $$module with coverage..."; \
		(cd $$module && \
			GOTOOLCHAIN=$(GO_TOOLCHAIN) go clean -testcache && \
			GOTOOLCHAIN=$(GO_TOOLCHAIN) go test -v -coverpkg=./... ./... -coverprofile $(COVERAGE_PATH)$$name.txt -count=1 -timeout=600s -tags=debug && \
			GOTOOLCHAIN=$(GO_TOOLCHAIN) go tool cover -func=$(COVERAGE_PATH)$$name.txt -o $(COVERAGE_PATH)$$name-functions.txt && \
			GOTOOLCHAIN=$(GO_TOOLCHAIN) go tool cover -html=$(COVERAGE_PATH)$$name.txt -o $(COVERAGE_PATH)$$name.html) || exit $$?; \
	done

.PHONY: test-race
test-race:
	@echo "Running tests with race detector..."
	@for module in $(MODULES); do \
		echo "Testing $$module with race detector..."; \
		(cd $$module && \
			GOTOOLCHAIN=$(GO_TOOLCHAIN) go clean -testcache && \
			GOTOOLCHAIN=$(GO_TOOLCHAIN) go test ./... -race -count=1 -timeout=600s) || exit $$?; \
	done

.PHONY: verify
verify: lint test-race
	@echo "All verifications passed successfully."