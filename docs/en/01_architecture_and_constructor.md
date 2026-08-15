# 01. Architecture, Constructor Execution Code, and Physical Memory Layout

## 1. Physical Memory Layout (Memory Layout & Alignment)

To prevent the inter-core **False Sharing (Cache Line Bouncing)** effect during parallel execution of atomic operations, structural elements of the buffer are aligned using adaptive padding `cpu.CacheLinePad` (package `golang.org/x/sys/cpu`). This ensures compatibility with both **x86-64** architectures (cache line size 64B) and **ARM64** (cache line size 128B, e.g., Apple Silicon M1-M4, AWS Graviton).

---

### 1.1 Physical Layout of `SPSCRingBuffer[T]` (Single-Producer Single-Consumer)

In the SPSC pattern, the data array is maximally dense (*Compact Dense Array*) to maximize spatial locality (*Spatial Locality*) and utilize the hardware prefetcher (*Hardware Prefetcher*). Atomic variables of the producer and consumer are placed on separate cache lines along with local shadow counters (*Shadow Counters*).

| Offset (ARM64 / 128B) | Offset (x86-64 / 64B) | Structure Field | Size | Purpose and Cache Line Isolation |
| :--- | :--- | :--- | :--- | :--- |
| `0x000 .. 0x007` | `0x00 .. 0x07` | `tail atomic.Uint64` | 8 B | **Producer Hot Path:** Write position |
| `0x008 .. 0x00F` | `0x08 .. 0x0F` | `headCache uint64` | 8 B | Shadow local copy of `head` (without Atomics) |
| `0x010 .. 0x08F` | `0x10 .. 0x4F` | `_ cpu.CacheLinePad` | 128B / 64B | **Cache Line 1:** Isolation of Producer hot path |
| `0x090 .. 0x097` | `0x50 .. 0x57` | `head atomic.Uint64` | 8 B | **Consumer Hot Path:** Read position |
| `0x098 .. 0x09F` | `0x58 .. 0x5F` | `tailCache uint64` | 8 B | Shadow local copy of `tail` (without Atomics) |
| `0x0A0 .. 0x11F` | `0x60 .. 0x9F` | `_ cpu.CacheLinePad` | 128B / 64B | **Cache Line 2:** Isolation of Consumer hot path |
| `0x120 .. 0x127` | `0xA0 .. 0xA7` | `mask uint64` | 8 B | Read-Only capacity configuration (`capacity - 1`) |
| `0x128 .. 0x12B` | `0xA8 .. 0xAB` | `state atomic.Uint32` | 4 B | Atomic buffer state flag |
| `0x12C .. 0x1AB` | `0xAC .. 0xEB` | `_ cpu.CacheLinePad` | 128B / 64B | **Cache Line 3:** Configuration isolation |
| `0x1AC .. 0x1C3` | `0xEC .. 0x103` | `ring []T` | 24 B | Slice descriptor of pointers (Pointer, Len, Cap) |

---

### 1.2 Physical Layout of `MPMCRingBuffer[T]` and `SlotPadded[T]` (Multi-Producer Multi-Consumer)

In the MPMC pattern using Dmitry Vyukov's algorithm, concurrent producers and consumers simultaneously update atomic `sequence` values in adjacent cells. To prevent cascading RFO storms (*Request For Ownership*) between CPU cores, each array slot is guaranteed to be separated by padding.

#### Slot structure layout (`SlotPadded[T]`):

```go
type SlotPadded[T any] struct {
	sequence atomic.Uint64 // 8 Bytes (Atomic barrier state)
	val      T             // N Bytes (Data of type T)
	_        cpu.CacheLinePad // 128B (ARM64) / 64B (x86-64)
}
```

#### Main buffer layout (`MPMCRingBuffer[T]`):

| Offset (ARM64 / 128B) | Offset (x86-64 / 64B) | Structure Field | Size | Purpose and Cache Line Isolation |
| :--- | :--- | :--- | :--- | :--- |
| `0x000 .. 0x007` | `0x00 .. 0x07` | `head atomic.Uint64` | 8 B | **Cache Line 1:** Atomic consumer head |
| `0x008 .. 0x087` | `0x08 .. 0x47` | `_ cpu.CacheLinePad` | 128B / 64B | Isolation of `head` from other fields |
| `0x088 .. 0x08F` | `0x48 .. 0x4F` | `tail atomic.Uint64` | 8 B | **Cache Line 2:** Atomic producer tail |
| `0x090 .. 0x10F` | `0x50 .. 0x8F` | `_ cpu.CacheLinePad` | 128B / 64B | Isolation of `tail` from other fields |
| `0x110 .. 0x117` | `0x90 .. 0x97` | `mask uint64` | 8 B | Read-Only capacity configuration (`capacity - 1`) |
| `0x118 .. 0x11B` | `0x98 .. 0x9B` | `state atomic.Uint32` | 4 B | Atomic buffer state flag |
| `0x11C .. 0x19B` | `0x9C .. 0xDB` | `_ cpu.CacheLinePad` | 128B / 64B | **Cache Line 3:** Configuration isolation |
| `0x19C .. 0x1B3` | `0xDC .. 0xF3` | `slots []SlotPadded[T]` | 24 B | Descriptor of isolated slots array |

---

## 2. Step-by-Step Constructor Execution Code

Below is the execution of constructors when requesting capacity $N = 5$ for type $T = \\text{int}$ (`NewSPSCRingBuffer[int](5)` and `NewMPMCRingBuffer[int](5)`).

### 2.1 Constructor `NewSPSCRingBuffer[T]`

```go
func NewSPSCRingBuffer[T any](capacity uint64) (*SPSCRingBuffer[T], error) {
	// Input: capacity = 5

	// Step 1: Boundary validation
	if 5 == 0 { return nil, ErrInvalidCapacity } // false
	if 5 > maxCapacity { return nil, ErrCapacityTooLarge } // false (5 <= 2^62)

	// Step 2: Rounding to power-of-two (Power-of-Two Alignment)
	// (5 & (5 - 1)) => (5 & 4) = 4 != 0 (value is not a power of two)
	if (5 & (5 - 1)) != 0 {
		// bits.Len64(5 - 1) = bits.Len64(4) = 3 (binary "100" has length 3 bits)
		// 1 << 3 = 8 (00001000_2)
		capacity = 1 << uint64(bits.Len64(5-1)) // capacity = 8
	}

	// Step 3: Bitwise mask calculation
	// mask = 8 - 1 = 7 (00000111_2)
	mask := capacity - 1

	// Step 4: Dense array allocation in heap (Heap Allocation)
	// 8 elements * 8 bytes/int = 64 bytes of clean data memory
	ring := make([]int, 8)

	// Step 5: Returning pointer (Escapes to Heap)
	return &SPSCRingBuffer[int]{
		mask: mask,
		ring: ring,
	}, nil
}
```

### 2.2 Constructor `NewMPMCRingBuffer[T]`

```go
func NewMPMCRingBuffer[T any](capacity uint64) (*MPMCRingBuffer[T], error) {
	// Input: capacity = 5

	if 5 == 0 { return nil, ErrInvalidCapacity }
	if 5 > maxCapacity { return nil, ErrCapacityTooLarge }

	// Rounding: 5 -> 8 (2^3)
	if (5 & (5 - 1)) != 0 {
		capacity = 1 << uint64(bits.Len64(5-1)) // capacity = 8
	}

	mask := capacity - 1

	// Step 1: Allocation of aligned SlotPadded array in heap
	// 8 slots * 144B/slot (on ARM64) = 1152 bytes in Heap
	slots := make([]SlotPadded[int], 8)

	// Step 2: Sequence invariant initialization (Vyukov Sequence State)
	// BCE Hint: Creating local slice to eliminate bounds checks (Bounds Check Elimination)
	subSlots := slots[:capacity]
	for i := uint64(0); i < capacity; i++ {
		// Setting initial barrier: slots[i].sequence = i
		// slots[0].sequence = 0, slots[1].sequence = 1, ..., slots[7].sequence = 7
		subSlots[i].sequence.Store(i)
	}

	return &MPMCRingBuffer[int]{
		mask:  mask,
		slots: slots,
	}, nil
}
```

---

## 3. Extended Q&A: System & Microarchitectural Analysis

### Q1: Why does the bitwise operation `index & mask` work identically to `index % capacity`, and why is it faster?

**Answer:**

When the buffer capacity $N$ is a power of two ($N = 2^k$), its binary representation looks like $100\\dots0_2$ with $k$ zeros. The mask $M = N - 1$ consists strictly of $k$ lower one bits ($011\\dots1_2$).

Applying the `AND` instruction with the mask zeros out all higher bits ($\\ge 2^k$), leaving only the lower $k$ bits, which is mathematically equivalent to the remainder of division $X \\pmod{2^k}$.

**Instruction-level CPU comparison:**
- **Integer division (`%`):** Translates to `DIV` / `IDIV` instruction (x86-64) or `SDIV` (ARM64). This instruction is non-pipelined (*Unpipelined Execution*) and takes 20–40 CPU cycles.
- **Bitwise `AND` (`&`):** Translates to the `AND` instruction. It executes in 1 CPU cycle in any ALU (*Arithmetic Logic Unit*) with throughput up to 4 instructions per cycle.

```text
Example for tail = 13, capacity = 8 (mask = 7):

  13 : 0000 1101_2
&  7 : 0000 0111_2  (mask = 8 - 1)
----------------
   5 : 0000 0101_2  (result: index 5)
```

Capacity rounding is performed via `bits.Len64(capacity - 1)`, which compiles to hardware leading-zero count instructions: `LZCNT`/`BSR` (x86-64) or `CLZ` (ARM64), executing in 1 cycle.

---

### Q2: Why did we abandon fixed padding `_ [56]byte` in favor of `cpu.CacheLinePad`?

**Answer:**

Using fixed arrays `_ [56]byte` relies on the outdated assumption that the $L1D$ cache line on all CPUs equals 64 bytes. This creates two critical problems:

1. **Microarchitectural incompatibility (ARM64 / Apple Silicon / IBM POWER):**
   On modern ARM64 processors (Apple M1/M2/M3/M4, Fujitsu A64FX), the $L1D$ cache line size or $L2$ fetch block is 128 bytes. When using `_ [56]byte`, the offsets of `head` and `tail` fall within a single 128-byte cache line. This leads to hidden False Sharing and performance degradation under load.

2. **Alignment drift errors:**
   The sum of field sizes during manual calculation often causes overflow. For example, `mask uint64` (8B) + `state atomic.Uint32` (4B) + `_ [56]byte` = 68 bytes. This overflows the 64B cache line and shifts all subsequent fields.

`cpu.CacheLinePad` at compile time substitutes an array of `_ [64]byte` for `GOARCH=amd64` or `_ [128]byte` for `GOARCH=arm64`, establishing an exact cross-platform isolation barrier.

---

### Q3: Why do SPSC and MPMC require fundamentally different memory layouts (Spatial Locality vs False Sharing)?

**Answer:**

The design is based on a fundamental engineering trade-off between data density in the L1 cache and inter-core contention (*Contention*):

- **SPSC (Single-threaded write and read):**
  Contention for slots between different cores is absent by definition. Using padding inside the array is harmful: it bloats the buffer size and causes cascading data evictions from the L1D cache (*L1 Cache Evictions*). A dense array `[]T` allows fitting 8-16 elements into a single cache line, maximizing the efficiency of the hardware stream prefetcher (*Hardware Stream Prefetcher*).

- **MPMC (Multi-threaded write and read):**
  Multiple cores simultaneously execute CAS and Atomic Store on adjacent array elements. If slots reside in the same cache line, the winning core sends an RFO signal (*Request For Ownership*) to the Interconnect bus, invalidating this line in the L1D caches of all other cores (*Cache Line Bouncing*). For MPMC, we deliberately sacrifice RAM density ($87.5\\%$ of the structure volume goes to `SlotPadded` padding) to isolate the atomic state of slots and completely eliminate the RFO storm.

---

### Q4: Why does the Go analyzer (`-m -m`) show that the buffer and its slices escape to the heap (Escape Analysis), and is this a problem?

**Answer:**

The `gc` compiler escape analysis shows:

```text
./buffer/ring_buffer.go:80:9: &buffer.SPSCRingBuffer[...]{...} escapes to heap
./buffer/ring_buffer.go:82:13: make([]int, capacity) escapes to heap
```

**Reason for escape:** Constructors return a pointer to the structure (`*SPSCRingBuffer`). Since the lifetime of the created object exceeds the duration of the constructor's stack frame (*Stack Frame Scope*), the compiler is forced to allocate memory on the Heap. The `make()` slice is written inside this structure and transitively escapes with it.

**Impact assessment:** This is desired and optimal behavior. Ring buffers are fundamental long-lived system objects (*Long-Lived Infrastructure Entities*). They are initialized once at application startup (*Warmup Phase*). Since no new allocations are created during `Push`/`Pop` execution (*Zero-Allocation Runtime Contract*), the one-time escape during construction has zero impact on GC Pauses and hot path latency (*Hot Path Latency*).

---

### Q5: Why is the `sequence` initialization loop necessary for MPMC but not for SPSC?

**Answer:**

- **In SPSC:** The data array does not contain internal atomic counters. Synchronization is performed exclusively through external `head` and `tail` pointers. Go Runtime's automatic memory zeroing (`make([]T)`) is sufficient.

- **In MPMC (Vyukov's algorithm):** Each slot is controlled by its own sequence barrier `sequence atomic.Uint64`. The algorithm requires a strict initial invariant:
  $$\\text{slot}[i].\\text{sequence} = i \\quad \\forall i \\in [0, N-1]$$
  If the `sequence.Store(i)` loop is not executed, all slots will contain 0. When attempting to call `Push` for index $tail = 1$, the slot state check $diff = \\text{seq} - tail = 0 - 1 = -1$ will yield $diff < 0$. The buffer will perceive empty slots as "full" and enter a state of permanent deadlock (*Deadlock / False Full Condition*).
