package main

import (
	"fmt"
	"github.com/DenysSkobalo/ringo/buffer"
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
}

