ROOT_PATH := $(dir $(realpath $(lastword $(MAKEFILE_LIST))))
COVERAGE_PATH := $(ROOT_PATH).coverage/
BIN_PATH ?= $(ROOT_PATH)bin

GO_VERSION ?= 1.25.12
GO_TOOLCHAIN ?= go$(GO_VERSION)
GRPC_GO_VERSION ?= 1.26.0
GRPC_GO_TOOLCHAIN ?= go$(GRPC_GO_VERSION)

GO125_MODULES := . http aws
MODULES := $(GO125_MODULES) grpc

# Each module is checked with its own minimum toolchain unless overridden.
select-toolchain = toolchain="$(GO_TOOLCHAIN)"; \
	if [ "$$module" = grpc ]; then toolchain="$(GRPC_GO_TOOLCHAIN)"; fi

# Ensure our local tools are preferred
export PATH := $(ROOT_PATH)tools/bin:$(PATH)

include $(ROOT_PATH)tools/tools.mk

# Keep aggregate targets while allowing each toolchain group to run separately.
GROUP_TARGETS := tidy lint lint-fix test test-cov test-race verify
GO125_TARGETS := $(addsuffix -go125,$(GROUP_TARGETS))
GRPC_TARGETS := $(addsuffix -grpc,$(GROUP_TARGETS))

.PHONY: $(GO125_TARGETS) $(GRPC_TARGETS)
$(GO125_TARGETS): %-go125:
	$(MAKE) $* MODULES="$(GO125_MODULES)"

$(GRPC_TARGETS): %-grpc:
	$(MAKE) $* MODULES=grpc

.PHONY: tidy
tidy:
	@echo "Running go mod tidy..."
	@for module in $(MODULES); do \
		echo "Tidying $$module..."; \
		$(select-toolchain); \
		(cd $$module && \
			GOTOOLCHAIN=$$toolchain go mod tidy) || exit $$?; \
	done

.PHONY: lint
lint: install-golangci-lint
	@echo "Running Go linter..."
	@for module in $(MODULES); do \
		echo "Linting $$module..."; \
		$(select-toolchain); \
		(cd $$module && GOTOOLCHAIN=$$toolchain $(GOLANGCI_LINT) run) || exit $$?; \
	done

.PHONY: lint-fix
lint-fix: install-golangci-lint
	@echo "Running Go linter with fixes..."
	@for module in $(MODULES); do \
		echo "Linting $$module..."; \
		$(select-toolchain); \
		(cd $$module && GOTOOLCHAIN=$$toolchain $(GOLANGCI_LINT) run --fix) || exit $$?; \
	done

.PHONY: test
test:
	@echo "Running tests..."
	@for module in $(MODULES); do \
		echo "Testing $$module..."; \
		$(select-toolchain); \
		(cd $$module && \
			GOTOOLCHAIN=$$toolchain go test ./... -count=1 -timeout=600s) || exit $$?; \
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
		$(select-toolchain); \
		(cd $$module && \
			GOTOOLCHAIN=$$toolchain go test -v -coverpkg=./... ./... -coverprofile $(COVERAGE_PATH)$$name.txt -count=1 -timeout=600s && \
			GOTOOLCHAIN=$$toolchain go tool cover -func=$(COVERAGE_PATH)$$name.txt -o $(COVERAGE_PATH)$$name-functions.txt && \
			GOTOOLCHAIN=$$toolchain go tool cover -html=$(COVERAGE_PATH)$$name.txt -o $(COVERAGE_PATH)$$name.html) || exit $$?; \
	done

.PHONY: test-race
test-race:
	@echo "Running tests with race detector..."
	@for module in $(MODULES); do \
		echo "Testing $$module with race detector..."; \
		$(select-toolchain); \
		(cd $$module && \
			GOTOOLCHAIN=$$toolchain go test ./... -race -count=1 -timeout=600s) || exit $$?; \
	done

.PHONY: verify
verify: lint test-race
	@echo "All verifications passed successfully."
