# ==============================================================================
# Project Configuration & Variables
# ==============================================================================
BINARY_NAME := playground
BUILD_DIR   := build
CMD_PATH    := ./examples/main.go

# Go Build Flags & Toolchain Variables
GO          := go
GOFLAGS     := -v
LDFLAGS     := -s -w

# Output Color Schemes
CYAN        := \033[0;36m
GREEN       := \033[0;32m
YELLOW      := \033[0;33m
RED         := \033[0;31m
RESET       := \033[0m

.PHONY: all help dev run build test bench profile race-check fmt lint vet clean check-deps

all: help

## help: Display all available workspace targets
help:
	@echo "$(CYAN)Local Go Playground Environment$(RESET)"
	@echo ""
	@echo "$(GREEN)Targets:$(RESET)"
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':' | sed -e 's/^/ /'

## run: Instantly execute main entry point via 'go run'
run: check-deps
	@echo "$(CYAN)[INFO] Executing playground code...$(RESET)"
	@$(GO) run $(CMD_PATH)

## dev: Run live-reloading server/playground via Air (if installed) or fallback to loop
dev: check-deps
	@if command -v air >/dev/null 2>&1; then \
		echo "$(CYAN)[INFO] Starting live-reloading watcher via Air...$(RESET)"; \
		air; \
	else \
		echo "$(YELLOW)[WARN] 'air' is not installed. Falling back to native execution.$(RESET)"; \
		echo "$(YELLOW)[HINT] Install air: go install github.com/air-verse/air@latest$(RESET)"; \
		$(GO) run $(CMD_PATH); \
	fi

## build: Compile native binary with stripped symbols (-s -w)
build: check-deps
	@echo "$(CYAN)[INFO] Compiling native binary to $(BUILD_DIR)/$(BINARY_NAME)...$(RESET)"
	@mkdir -p $(BUILD_DIR)
	@$(GO) build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_PATH)
	@echo "$(GREEN)[SUCCESS] Binary successfully compiled: $(BUILD_DIR)/$(BINARY_NAME)$(RESET)"
	@ls -lh $(BUILD_DIR)/$(BINARY_NAME) | awk '{print "$(YELLOW)[SIZE] Executable Size: " $$5 "$(RESET)"}'

## test: Run unit tests with race detection (-race)
test: check-deps
	@echo "$(CYAN)[INFO] Running tests with Data Race Detector...$(RESET)"
	@$(GO) test -v -race -count=1 ./...

## bench: Execute performance benchmarks with zero-allocation tracking
bench: check-deps
	@echo "$(CYAN)[INFO] Running benchmarks (-benchmem -bench .)...$(RESET)"
	@$(GO) test -v -bench=. -benchmem -run=^$$ ./...

## profile: Generate pprof CPU and Memory profiles for performance analysis
profile: check-deps
	@echo "$(CYAN)[INFO] Generating pprof memory and CPU profiles...$(RESET)"
	@mkdir -p $(BUILD_DIR)/pprof
	@$(GO) test -v -bench=. -benchmem -cpuprofile=$(BUILD_DIR)/pprof/cpu.pprof -memprofile=$(BUILD_DIR)/pprof/mem.pprof -run=^$$ ./...
	@echo "$(GREEN)[SUCCESS] Profiles generated in $(BUILD_DIR)/pprof/$(RESET)"
	@echo "$(YELLOW)[HINT] Analyze CPU profile: go tool pprof -http=:8080 $(BUILD_DIR)/pprof/cpu.pprof$(RESET)"

## fmt: Auto-format source code according to gofmt and goimports standards
fmt: check-deps
	@echo "$(CYAN)[INFO] Formatting code via gofmt...$(RESET)"
	@$(GO) fmt ./...
	@if command -v goimports >/dev/null 2>&1; then \
		echo "$(CYAN)[INFO] Sorting imports via goimports...$(RESET)"; \
		goimports -w .; \
	fi

## vet: Execute static code analysis (go vet and golangci-lint)
vet: check-deps
	@echo "$(CYAN)[INFO] Executing go vet...$(RESET)"
	@$(GO) vet ./...
	@if command -v golangci-lint >/dev/null 2>&1; then \
		echo "$(CYAN)[INFO] Running golangci-lint...$(RESET)"; \
		golangci-lint run; \
	fi

## check-deps: Verify Go installation and toolchain availability
check-deps:
	@command -v $(GO) >/dev/null 2>&1 || { echo "$(RED)[ERROR] Go toolchain is not installed. Aborting.$(RESET)"; exit 1; }

## clean: Remove build artifacts, pprof dumps, and binary outputs
clean:
	@echo "$(YELLOW)[INFO] Cleaning build artifacts and profile dumps...$(RESET)"
	@rm -rf $(BUILD_DIR)
	@echo "$(GREEN)[SUCCESS] Cleanup completed.$(RESET)"
