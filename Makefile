# ==============================================================================
# Project Configuration & Variables
# ==============================================================================
BINARY_NAME := ringbuffer
BUILD_DIR   := build
PPROF_DIR   := $(BUILD_DIR)/pprof
CMD_PATH    := ./examples/main.go

# Performance & Benchmarking Parameters
BENCH_TIME  := 3s
PPROF_PORT  := 8080

# Go Toolchain Flags
GO          := go
GOFLAGS     := -v
LDFLAGS     := -s -w

# Output Color Schemes
CYAN        := \033[0;36m
GREEN       := \033[0;32m
YELLOW      := \033[0;33m
RED         := \033[0;31m
RESET       := \033[0m

.PHONY: all help dev run build test bench profile pprof-cpu pprof-mem escape fmt vet clean check-deps

all: help

## help: Display all available workspace targets
help:
	@echo "$(CYAN)Lock-Free RingBuffer Development Environment$(RESET)"
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
	@echo "$(CYAN)[INFO] Running unit tests with Data Race Detector...$(RESET)"
	@$(GO) test -v -race -count=1 ./...

## bench: Execute performance benchmarks with zero-allocation tracking
bench: check-deps
	@echo "$(CYAN)[INFO] Running benchmarks (benchtime=$(BENCH_TIME))...$(RESET)"
	@$(GO) test -v -bench=. -benchmem -benchtime=$(BENCH_TIME) -run=^$$ ./...

## profile: Generate CPU and Memory pprof profile dumps from benchmarks
profile: check-deps
	@echo "$(CYAN)[INFO] Generating CPU and Memory profiles in $(PPROF_DIR)...$(RESET)"
	@mkdir -p $(PPROF_DIR)
	@$(GO) test -v -bench=. -benchmem -benchtime=$(BENCH_TIME) \
		-cpuprofile=$(PPROF_DIR)/cpu.pprof \
		-memprofile=$(PPROF_DIR)/mem.pprof \
		-run=^$$ ./buffer
	@echo "$(GREEN)[SUCCESS] Profiles stored in $(PPROF_DIR)/$(RESET)"
	@echo "$(YELLOW)[HINT] Run 'make pprof-cpu' or 'make pprof-mem' to open web UI.$(RESET)"

## pprof-cpu: Launch interactive pprof Web UI for CPU profile
pprof-cpu: check-deps
	@if [ ! -f $(PPROF_DIR)/cpu.pprof ]; then \
		echo "$(RED)[ERROR] CPU profile not found. Run 'make profile' first.$(RESET)"; exit 1; \
	fi
	@echo "$(CYAN)[INFO] Launching pprof Web Server on http://localhost:$(PPROF_PORT)...$(RESET)"
	@$(GO) tool pprof -http=:$(PPROF_PORT) $(PPROF_DIR)/cpu.pprof

## pprof-mem: Launch interactive pprof Web UI for Memory profile
pprof-mem: check-deps
	@if [ ! -f $(PPROF_DIR)/mem.pprof ]; then \
		echo "$(RED)[ERROR] Memory profile not found. Run 'make profile' first.$(RESET)" ; exit 1; \
	fi
	@echo "$(CYAN)[INFO] Launching pprof Web Server on http://localhost:$(PPROF_PORT)...$(RESET)"
	@$(GO) tool pprof -http=:$(PPROF_PORT) $(PPROF_DIR)/mem.pprof

## escape: Inspect compiler decisions (Escape Analysis & Bounds Check Elimination)
escape: check-deps
	@echo "$(CYAN)[INFO] Running gc compiler escape analysis & BCE checks...$(RESET)"
	@$(GO) build -gcflags="-m -m -l" ./buffer/...

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
