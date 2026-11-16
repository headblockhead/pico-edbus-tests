package main

import "testing"

func TestSimpleTransmitReceive(t *testing.T) {
	setup, err := setupSimulation()
	if err != nil {
		t.Fatalf("Failed to setup simulation: %v", err)
	}

	nodeA := setup.nodes[0]

	aTXWriter := nodeA.smTX.FIFOTXWriter()
	// TODO
	aTXWriter.Write()
}
