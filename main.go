package main

import (
	"fmt"
	"time"

	"git.edwardh.dev/pio/instr"
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
		err = smTXA.FIFOTXWriter().Write(0xF0000000 + uint32(i))
		if err != nil {
			panic(err)
		}
	}

	for i := range 8 {
		_, _ = smTXB, i
		err = smTXB.FIFOTXWriter().Write(0x00000000 + uint32(i))
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

		if (pio0A.IRQs()>>0)&1 == 1 {
			println("RP2040 A: got IRQ 'error' on tick", ticks)
			smTXA.ClearFIFOs()
			smTXA.ForceInstruction(instr.NewSet(instr.SetDestinationX, 7).Encode() | instr.NewSideset(0b11).Encode(2, false))
			err := pio0A.SetIRQ(0, false)
			if err != nil {
				panic(err)
			}
		}
		if (pio0A.IRQs()>>1)&1 == 1 {
			println("RP2040 A: got IRQ 'done' on tick", ticks)
			smTXA.ForceInstruction(instr.NewSet(instr.SetDestinationX, 7).Encode() | instr.NewSideset(0b11).Encode(2, false))
			err := pio0A.SetIRQ(1, false)
			if err != nil {
				panic(err)
			}
		}
		if (pio0B.IRQs()>>0)&1 == 1 {
			println("RP2040 B: got IRQ 'error' on tick", ticks)
			smTXB.ClearFIFOs()
			smTXB.ForceInstruction(instr.NewSet(instr.SetDestinationX, 7).Encode() | instr.NewSideset(0b11).Encode(2, false))
			err := pio0B.SetIRQ(0, false)
			if err != nil {
				panic(err)
			}
		}
		if (pio0B.IRQs()>>1)&1 == 1 {
			println("RP2040 B: got IRQ 'done' on tick", ticks)
			smTXB.ForceInstruction(instr.NewSet(instr.SetDestinationX, 7).Encode() | instr.NewSideset(0b11).Encode(2, false))
			err := pio0B.SetIRQ(1, false)
			if err != nil {
				panic(err)
			}
		}

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
