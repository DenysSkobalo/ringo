# TODO: Ringo Development Roadmap

This document outlines the technical roadmap for **Ringo** — a high-performance concurrency toolkit designed for non-blocking and blocking memory buffer pipelines in Go.

---

## 🏗 Phase 1: Project Restructuring & Core Setup

- [ ] **Module Reorganization**
  - [ ] Rename module in `go.mod` from `playground-golang` to `github.com/user/ringo` (or organizational path).
  - [ ] Extract core logic from `main.go` into dedicated packages: `buffer`, `pool`, `logger`.
  - [ ] Move execution sandbox and demo code to `examples/main.go`.

- [ ] **Logger Package (`logger`)**
  - [ ] Add unit tests covering logger creation across configurations (`WithLevel`, `WithAddSource`, `WithJSON`).
  - [ ] Implement support for dynamic runtime log level adjustments via `slog.LevelVar`.

---

## ⚡ Phase 2: Non-Blocking Engine

Non-blocking operations guarantee sub-microsecond latency execution paths and provide proactive backpressure handling (Load Shedding / Fast-Fail).

- [ ] **Low-Level Primitives (`buffer/primitives.go`)**
  - [ ] Export `TrySet[T any](ch chan<- T, val T) error` for zero-allocation sends.
  - [ ] Export `TryGet[T any](ch <-chan T) (T, bool)` for zero-allocation receives.
  - [ ] Implement `TrySetBatch[T any](ch chan<- T, items []T) (int, error)` for bulk non-blocking writes.
  - [ ] Implement `TryGetBatch[T any](ch <-chan T, maxItems int) ([]T, int)` for bulk non-blocking reads.

- [ ] **RingBuffer Structure (`buffer/ring_buffer.go`)**
  - [ ] Implement constructor `NewRingBuffer[T any](capacity int) (*RingBuffer[T], error)` with strict capacity bounds checks (`capacity > 0`).
  - [ ] Implement non-blocking `TryPush(val T) error` (returns `ErrBufferFull` when the underlying `hchan` buffer is at capacity).
  - [ ] Implement non-blocking `TryPop() (T, bool)` (returns `zero, false` when empty).
  - [ ] Implement lock-free state inspectors: `Len() int` (`hchan.qcount`) and `Cap() int` (`hchan.dataqsiz`).
  - [ ] Implement thread-safe buffer shutdown via `Close() error`.

---

## 🛑 Phase 3: Blocking Engine & Backpressure Management

Blocking operations guarantee zero data loss and enforce natural flow control (Backpressure) when operating under constrained consumer throughput.

- [ ] **Context-Aware Primitives (`buffer/primitives.go`)**
  - [ ] Implement `PushContext[T any](ctx context.Context, ch chan<- T, val T) error`:
    - Blocking send with cancellation propagation via `ctx.Done()`.
    - Proper error handling for `context.Canceled` and `context.DeadlineExceeded`.
  - [ ] Implement `PopContext[T any](ctx context.Context, ch <-chan T) (T, error)`:
    - Blocking receive with cancellation propagation via `ctx.Done()`.

- [ ] **Blocking RingBuffer Methods**
  - [ ] Implement `Push(ctx context.Context, val T) error`:
    - Blocks until buffer capacity opens up or `ctx` signals cancellation.
  - [ ] Implement `Pop(ctx context.Context) (T, error)`:
    - Blocks until data becomes available or `ctx` signals cancellation.

- [ ] **Worker Pool Pipeline (`pool/worker_pool.go`)**
  - [ ] Implement `ConsumerPool[T any]` for concurrent data processing pipeline execution.
  - [ ] Implement constructor `NewConsumerPool[T any](workers int, handler TaskHandler[T]) (*ConsumerPool[T], error)`.
  - [ ] Implement `Start(ctx context.Context, buf *RingBuffer[T]) error`:
    - Spawn $N$ worker goroutines ($G$).
    - Handle both buffer termination (`Close()`) and context signaling (`ctx.Done()`).
    - Guarantee zero goroutine leaks via `sync.WaitGroup` coordination.

---

## 📊 Phase 4: Memory Optimization & Profiling

- [ ] **Heap Allocation Controls**
  - [ ] Create benchmark suite `BenchmarkRingBuffer_TryPush` and `BenchmarkRingBuffer_TryPop`.
  - [ ] Enforce **`0 B/op`** target in hot paths during benchmarking (`make bench`).
  - [ ] Verify escape analysis boundaries using compiler flags:
    `go build -gcflags="-m -m"`

- [ ] **pprof Profiling**
  - [ ] Automate CPU and Memory profile dumps generation via `make profile`.
  - [ ] Eliminate allocations and lock contention in critical paths.

---

## 🧪 Phase 5: Quality Assurance & CI/CD

- [ ] **Unit Testing (>90% Coverage Target)**
  - [ ] Cover edge cases: send on closed channel, receive on `nil` channel, buffer overflow/underflow.
  - [ ] Implement concurrent integration tests for Many-to-Many producer-consumer workloads.

- [ ] **Data Race Detection**
  - [ ] Enforce strict `-race` flag validation across all test targets (`go test -race -count=1 ./...`).

- [ ] **Automated CI Workflows**
  - [ ] Configure `.github/workflows/ci.yml`.
  - [ ] Add static analysis step via `golangci-lint`.
  - [ ] Configure matrix builds testing across Linux, macOS, and Windows on Go 1.22+.
