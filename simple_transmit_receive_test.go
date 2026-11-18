package main

import "testing"

func TestSimpleTransmitReceive(t *testing.T) {
	setup, err := setupSimulation()
	if err != nil {
		t.Fatalf("Failed to setup simulation: %v", err)
	}

	nodeA := setup.nodes[0]

	aTXWriter := nodeA.smTX.FIFOTXWriter()
	err = aTXWriter.Write(0x0AF0BEEF)
	if err != nil {
		t.Fatalf("Failed to write to TX FIFO: %v", err)
	}

}
