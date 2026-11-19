package main

import (
	"slices"
	"testing"
)

func TestSimpleTransmitReceive(t *testing.T) {
	setup, err := setupSimulation()
	if err != nil {
		t.Fatalf("failed to setup simulation: %v", err)
	}

	nodeA := setup.nodes[0]

	aTXWriter := nodeA.smTX.FIFOTXWriter()

	testMessage := []uint32{0x00000000, 0xFFFFFFFF, 0x00FFAAAA, 0xDEADBEEF, 0xEDB05501, 0x12345678, 0xCAFECAFE, 0xABCDEF99}

	for i, word := range testMessage {
		err = aTXWriter.Write(word)
		if err != nil {
			t.Fatalf("failed to write word %d to TX FIFO: %v", i, err)
		}
	}

	rxBuffers := make([][]uint32, len(setup.nodes))

	for cycle := range 5_000 {
		err := setup.sim.Tick()
		if err != nil {
			t.Fatalf("error whilst ticking simulation: %v", err)
		}

		for i, node := range setup.nodes {
			if (node.pio.IRQs()>>0)&1 == 1 {
				onErrorIRQ(node.smTX)
				err := node.pio.SetIRQ(0, false)
				if err != nil {
					t.Fatalf("failed to clear node [%s]'s error IRQ: %v", node.r.Label(), err)
				}
				t.Errorf("error: node [%s] raised an unexpected error IRQ at cycle %d", node.r.Label(), cycle)
			}
			if (node.pio.IRQs()>>1)&1 == 1 {
				onDoneIRQ(node.smTX)
				err := node.pio.SetIRQ(1, false)
				if err != nil {
					t.Fatalf("failed to clear node [%s]'s done IRQ: %v", node.r.Label(), err)
				}
				t.Logf("node [%s] raised a done IRQ at cycle %d", node.r.Label(), cycle)
			}

			if !node.smRX.FIFORXReader().IsEmpty() {
				word, err := node.smRX.FIFORXReader().Read()
				if err != nil {
					t.Fatalf("failed to read word from RX FIFO of node [%s]: %v", node.r.Label(), err)
				}
				t.Logf("node [%s] received word: 0x%08X at cycle %d", node.r.Label(), word, cycle)
				rxBuffers[i] = append(rxBuffers[i], word)
			}
		}
	}

	for i, node := range setup.nodes {
		if slices.Compare(rxBuffers[i], testMessage) != 0 {
			t.Errorf("error: node [%s] received incorrect data. Expected: %08X, Got: %08X", node.r.Label(), testMessage, rxBuffers[i])
		}
	}
}
