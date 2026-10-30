package main

import (
	"fmt"
	"time"
)

func main() {
	s, ebdNet, ebcNet, rA, rB, err := setupSimulation()
	if err != nil {
		panic(err)
	}

	pio0A, err := rA.PIO(0)
	if err != nil {
		panic(err)
	}
	pio0B, err := rB.PIO(0)
	if err != nil {
		panic(err)
	}

	smTXA, err := pio0A.StateMachine(0)
	if err != nil {
		panic(err)
	}
	smTXB, err := pio0B.StateMachine(0)
	if err != nil {
		panic(err)
	}
	smRXA, err := pio0A.StateMachine(1)
	if err != nil {
		panic(err)
	}
	smRXB, err := pio0B.StateMachine(1)
	if err != nil {
		panic(err)
	}

	for i := range 8 {
		_, _ = smTXA, i
		err = smTXA.FIFOTXWriter().Write(0xF0000AAA + uint32(i))
		if err != nil {
			panic(err)
		}
	}

	for i := range 8 {
		_, _ = smTXB, i
		err = smTXB.FIFOTXWriter().Write(0xAAAAAAAA + uint32(i))
		if err != nil {
			panic(err)
		}
	}

	ticks := 0
	for {
		err := s.Tick()
		if err != nil {
			panic(err)
		}

		_, _ = ebcNet, ebdNet

		if !smRXA.FIFORXReader().IsEmpty() {
			v, err := smRXA.FIFORXReader().Read()
			if err != nil {
				panic(err)
			}
			fmt.Printf("RX A on tick %d: %08X\n", ticks, v)
		}
		if !smRXB.FIFORXReader().IsEmpty() {
			v, err := smRXB.FIFORXReader().Read()
			if err != nil {
				panic(err)
			}
			fmt.Printf("RX B on tick %d: %08X\n", ticks, v)
		}

		time.Sleep(0 * time.Millisecond)
		ticks++
	}
}
