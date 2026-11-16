package main

import (
	"errors"

	"git.edwardh.dev/pio"
	"git.edwardh.dev/pio/gpio"
	"git.edwardh.dev/pio/instr"
	"git.edwardh.dev/pio/memory"
	"git.edwardh.dev/pio/pioblock"
	"git.edwardh.dev/pio/simulation"
	"git.edwardh.dev/pio/statemachine"
)

var systemClock float32 = (125 * 1000 * 1000)

var edbusClock float32 = (125 * 1000 * 1000)

var programTX = memory.NewProgram([]uint16{
	instr.NewPushPull(true, false, true).Encode() | instr.NewSideset(0b00).Encode(2, false),   //  0: pull   block           side 0
	instr.NewOut(instr.OutDestinationY, 1).Encode() | instr.NewSideset(0b00).Encode(2, false), //  1: out    y, 1            side 0
	instr.NewJump(instr.JumpYZero, 9).Encode() | instr.NewSideset(0b00).Encode(2, false),      //  2: jmp    !y, 9           side 0
	instr.EncodeNOP() | instr.NewSideset(0b11).Encode(2, false) | instr.NewDelay(3).Encode(),  //  3: nop                    side 3 [3]
	instr.NewJump(instr.JumpPin, 10).Encode() | instr.NewSideset(0b11).Encode(2, false),       //  4: jmp    pin, 10         side 3
	instr.NewIRQ(false, true, 0).Encode() | instr.NewSideset(0b11).Encode(2, false),           //  5: irq    wait 0          side 3
	//     .wrap_target
	instr.NewPushPull(true, false, true).Encode() | instr.NewSideset(0b11).Encode(2, false),                             //  6: pull   block           side 3
	instr.NewWait(false, instr.WaitSourceIRQ, 7).Encode() | instr.NewSideset(0b11).Encode(2, false),                     //  7: wait   0 irq, 7        side 3
	instr.NewJump(instr.JumpAlways, 10).Encode() | instr.NewSideset(0b01).Encode(2, false) | instr.NewDelay(7).Encode(), //  8: jmp    10              side 1 [7]
	instr.EncodeNOP() | instr.NewSideset(0b10).Encode(2, false) | instr.NewDelay(4).Encode(),                            //  9: nop                    side 2 [4]
	instr.NewJump(instr.JumpOSRENotEmpty, 1).Encode() | instr.NewSideset(0b00).Encode(2, false),                         // 10: jmp    !osre, 1        side 0
	instr.NewJump(instr.JumpXNonZeroThenDecrement, 0).Encode() | instr.NewSideset(0b00).Encode(2, false),                // 11: jmp    x--, 0          side 0
	instr.NewIRQ(false, true, 1).Encode() | instr.NewSideset(0b11).Encode(2, false) | instr.NewDelay(7).Encode(),        // 12: irq    wait 1          side 3 [7]
	//     .wrap
})

var programTXEntryPoint uint = 6
var programTXWrapFrom uint = 12
var programTXWrapTo uint = 6

var programRX = memory.NewProgram([]uint16{
	instr.EncodeNOP(),
})

var programRXWrapFrom uint = 0
var programRXWrapTo uint = 0

var ebdPinNum uint = 6
var ebcPinNum uint = 7
var edbusPIONum uint = 0
var txSMNum uint = 0
var rxSMNum uint = 1

func setupTX(r *pio.RP2040) error {
	if ebdPinNum+1 != ebcPinNum {
		return errors.New("ebd+1 != ebc")
	}

	pio, err := r.PIO(edbusPIONum)
	if err != nil {
		return err
	}

	memoryWriter := pio.MemoryWriter()
	offset, err := memoryWriter.AddProgram(programTX)
	if err != nil {
		return err
	}

	sm, err := pio.StateMachine(txSMNum)
	if err != nil {
		return err
	}

	err = sm.SetWrap(programTXWrapTo+offset, programTXWrapFrom+offset)
	if err != nil {
		return err
	}
	err = sm.SetJumpPin(ebdPinNum)
	if err != nil {
		return err
	}
	err = sm.SetBaseSidesetPin(ebdPinNum)
	if err != nil {
		return err
	}
	err = sm.SetSideset(2, false, true)
	if err != nil {
		return err
	}
	err = sm.SetFIFOJoin(statemachine.FIFOJoinTX)
	if err != nil {
		return err
	}
	err = sm.SetClockDivisor(systemClock / edbusClock)
	if err != nil {
		return err
	}
	err = sm.SetOutShift(false, false, 32)
	if err != nil {
		return err
	}

	err = pio.SetPinOutput(ebdPinNum, true)
	if err != nil {
		return err
	}
	err = pio.SetPinOutputEnable(ebdPinNum, true)
	if err != nil {
		return err
	}
	err = pio.SetPinOutput(ebcPinNum, true)
	if err != nil {
		return err
	}
	err = pio.SetPinOutputEnable(ebcPinNum, true)
	if err != nil {
		return err
	}

	var gpioFunction gpio.Function
	switch edbusPIONum {
	case 0:
		gpioFunction = gpio.FunctionPIO0
	case 1:
		gpioFunction = gpio.FunctionPIO1
	default:
		return errors.New("unknown pio")
	}
	ebdG, err := r.GPIO(ebdPinNum)
	if err != nil {
		return err
	}
	ebdP, err := r.Pad(ebdPinNum)
	if err != nil {
		return err
	}
	ebcG, err := r.GPIO(ebcPinNum)
	if err != nil {
		return err
	}
	ebcP, err := r.Pad(ebcPinNum)
	if err != nil {
		return err
	}

	ebdG.SetFunction(gpioFunction)
	ebdG.SetOutputEnableOverride(gpio.OverrideInvert)
	ebdP.SetPullDown(false)
	ebdP.SetPullUp(true)
	ebcG.SetFunction(gpioFunction)
	ebcG.SetOutputEnableOverride(gpio.OverrideInvert)
	ebcP.SetPullDown(false)
	ebcP.SetPullUp(true)

	err = pio.SetPinOutput(ebdPinNum, false)
	if err != nil {
		return err
	}
	err = pio.SetPinOutput(ebcPinNum, false)
	if err != nil {
		return err
	}

	sm.Init(offset + programTXEntryPoint)
	sm.ForceInstruction(instr.NewSet(instr.SetDestinationX, 7).Encode() | instr.NewSideset(0b11).Encode(2, false))
	sm.SetEnabled(true)

	return nil
}

func setupRX(r *pio.RP2040) error {
	if ebdPinNum+1 != ebcPinNum {
		return errors.New("ebd+1 != ebc")
	}

	pio, err := r.PIO(edbusPIONum)
	if err != nil {
		return err
	}

	memoryWriter := pio.MemoryWriter()
	offset, err := memoryWriter.AddProgram(programRX)
	if err != nil {
		return err
	}

	sm, err := pio.StateMachine(rxSMNum)
	if err != nil {
		return err
	}

	err = sm.SetWrap(programRXWrapTo+offset, programRXWrapFrom+offset)
	if err != nil {
		return err
	}
	err = sm.SetBaseInPin(ebdPinNum)
	if err != nil {
		return err
	}
	err = sm.SetJumpPin(ebcPinNum)
	if err != nil {
		return err
	}
	err = sm.SetFIFOJoin(statemachine.FIFOJoinRX)
	if err != nil {
		return err
	}
	err = sm.SetClockDivisor(systemClock / edbusClock)
	if err != nil {
		return err
	}
	err = sm.SetInShift(false, false, 32)
	if err != nil {
		return err
	}

	sm.Init(offset)
	sm.SetEnabled(true)

	return nil
}

func newEDBUSRP2040(label string) (*pio.RP2040, error) {
	r := pio.NewRP2040(label)

	err := setupTX(r)
	if err != nil {
		return nil, err
	}
	err = setupRX(r)
	if err != nil {
		return nil, err
	}

	return r, nil
}

type edbusSetupRP2040 struct {
	r    *pio.RP2040
	pio  *pioblock.PIOBlock
	smTX *statemachine.StateMachine
	smRX *statemachine.StateMachine
}

type edbusSetup struct {
	sim *simulation.Simulation

	ebd *simulation.Net
	ebc *simulation.Net

	nodes []edbusSetupRP2040
}

func setupSimulation() (edbusSetup, error) {
	var err error
	setup := edbusSetup{}

	setup.sim = simulation.New()

	setup.ebd = simulation.NewNet("EBD")
	err = setup.sim.AddNet(setup.ebd)
	if err != nil {
		return setup, err
	}

	setup.ebc = simulation.NewNet("EBC")
	err = setup.sim.AddNet(setup.ebc)
	if err != nil {
		return setup, err
	}

	for _, l := range []string{"A", "B", "C", "D", "E"} {
		node := edbusSetupRP2040{}

		node.r, err = newEDBUSRP2040(l)
		if err != nil {
			return setup, err
		}
		err = setup.sim.AddTicker(node.r)
		if err != nil {
			return setup, err
		}
		node.pio, err = node.r.PIO(edbusPIONum)
		if err != nil {
			return setup, err
		}
		node.smTX, err = node.pio.StateMachine(txSMNum)
		if err != nil {
			return setup, err
		}
		node.smRX, err = node.pio.StateMachine(rxSMNum)
		if err != nil {
			return setup, err
		}

		ebdCon, err := node.r.Connection(ebdPinNum)
		if err != nil {
			return setup, err
		}
		err = setup.ebd.Connect(ebdCon)
		if err != nil {
			return setup, err
		}
		ebcCon, err := node.r.Connection(ebcPinNum)
		if err != nil {
			return setup, err
		}
		err = setup.ebc.Connect(ebcCon)
		if err != nil {
			return setup, err
		}
	}

	return setup, nil
}

func onDoneIRQ(smTX *statemachine.StateMachine) {
	smTX.ForceInstruction(instr.NewSet(instr.SetDestinationX, 7).Encode() | instr.NewSideset(0b11).Encode(2, false))
}

func onErrorIRQ(smTX *statemachine.StateMachine) {
	smTX.ClearFIFOs()
	smTX.ForceInstruction(instr.NewSet(instr.SetDestinationX, 7).Encode() | instr.NewSideset(0b11).Encode(2, false))
}
