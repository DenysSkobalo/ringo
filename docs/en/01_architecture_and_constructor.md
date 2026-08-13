# 01. Architecture, Constructor Execution Trace, and Bitwise Masking

## 1. Physical Memory Layout (`RingBuffer[T]`)

To mitigate **False Sharing** under high contention, atomic variables (`head`, `tail`, `state`) are separated across distinct 64-byte CPU cache lines using explicit padding byte arrays (`_[56]byte`).

| Memory Offset | Field | Size | Cache Line & Purpose |
| :--- | :--- | :--- | :--- |
| `0x00 .. 0x37` | `_ [56]byte` | 56 B | Padding before `head` |
| `0x38 .. 0x3F` | `head atomic.Uint64` | 8 B | **Cache Line 1 (64 B):** Consumer read cursor |
| `0x40 .. 0x77` | `_ [56]byte` | 56 B | Padding between `head` and `tail` |
| `0x78 .. 0x7F` | `tail atomic.Uint64` | 8 B | **Cache Line 2 (64 B):** Producer write cursor |
| `0x80 .. 0xB7` | `_ [56]byte` | 56 B | Padding before configuration |
| `0xB8 .. 0xBF` | `mask uint64` | 8 B | **Cache Line 3 (64 B):** Bitwise mask (`capacity - 1`) |
| `0xC0 .. 0xC3` | `state atomic.Uint32`| 4 B | Atomic state flags (Closed / Draining) |
| `0xC4 .. 0xF3` | `_ [48]byte` | 48 B | Padding to align struct end (64 B total) |
| `0xF8 .. 0x10F`| `slots []Slot[T]` | 24 B | **Cache Line 4:** Slice header (Pointer, Len, Cap) |

---

## 2. Constructor Execution Trace

Below is a step-by-step execution trace when constructing a buffer with requested capacity $N = 5$ and item type $T = \text{int}$ (`NewRingBuffer[int](5)`):

```go
func NewRingBuffer[T any](capacity uint64) (*RingBuffer[T], error) {
    // Input: capacity = 5

    // Step 1: Validation
    if 5 == 0 { return nil, ErrInvalidCapacity } // false

    if 5 > maxCapacity { return nil, ErrCapacityTooLarge } // false (5 <= 2^62)

    // Step 2: Power-of-two check
    // (5 & (5 - 1)) => (5 & 4) => (00000101_2 & 00000100_2) = 00000100_2 (4 != 0)
    if (5 & (5 - 1)) != 0 {
        // Step 3: Round up to nearest power of two
        // bits.Len64(5 - 1) = bits.Len64(4) = 3 (binary "100" uses 3 bits)
        // 1 << 3 = 8 (00001000_2)
        capacity = 1 << uint64(bits.Len64(5-1)) // capacity is now 8
    }

    // Step 4: Calculate bitwise mask
    // mask = 8 - 1 = 7 (00000111_2)
    mask := capacity - 1

    // Step 5: Allocate memory for 8 slots
    // 8 slots * 16 bytes/slot = 128 bytes on heap
    slots := make([]Slot[int], 8)

    // Step 6: Pre-initialize sequence barriers
    // Loop i from 0 to 7:
    // slots[0].sequence = 0, slots[1].sequence = 1, ..., slots[7].sequence = 7
    for i := uint64(0); i < 8; i++ {
        slots[i].sequence.Store(i)
    }

    return &RingBuffer[int]{
        mask:  7,
        slots: slots,
    }, nil
}
```

## 3. Q&A: Why does mask = 7 work for any index?

### Question:
Why does the bitwise operation `index & mask` yield the exact array index for any growing integer (e.g., $13$), and why is it faster than modulo division?

### Answer:
When capacity $N$ is constrained to a power of two ($N = 2^k$), its mask $M = N - 1$ consists entirely of $k$ lower set bits ($1\text{s}$). Performing a bitwise AND with mask $M$ strips all higher bits ($\ge 2^k$) from the target number, leaving only the lower $k$ bits. Mathematically, this is strictly equivalent to modulo division $X \pmod{2^k}$, but executes in 1 CPU clock cycle (AND instruction) compared to 10–30 CPU clock cycles required for integer division (DIV instruction).

### Practical Example: Writing item #13 (tail = 13) into capacity N = 8:

* **Modulo Division (Slow):**
  `13 % 8 = 5` (або $13 \pmod 8 = 5$)

* **Bitwise AND with Mask M = 7 (Ultra-fast):**
  `13 & 7 = 5`

```text
  13 : 0000 1101 (binary representation)
&  7 : 0000 0111 (mask capacity - 1)
----------------
   5 : 0000 0101 (result: array index)
```

The operation maps $13 \to 5$ instantly at the hardware instruction level.
