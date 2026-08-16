package main

import (
	"fmt"
	"github.com/DenysSkobalo/ringo/buffer"
	"unsafe"
)

func main() {
	const cap = 5
	spscringbuf, err := buffer.NewSPSCRingBuffer[int](cap)
	if err != nil {
		panic(err)
	}

	mpmcringbuf, err := buffer.NewMPMCRingBuffer[int](cap)
	if err != nil {
		panic(err)
	}

	fmt.Println("SPSC:", spscringbuf)
	fmt.Println("MPMC:", mpmcringbuf)

	fmt.Printf("Slot size: %d bytes\n", unsafe.Sizeof(buffer.SlotPadded[int]{}))
	fmt.Printf("Total array size: %d KB\n", (uint64(unsafe.Sizeof(buffer.SlotPadded[int]{})) * cap) / 1024)
}

