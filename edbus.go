package main

import (
	"errors"

	"git.edwardh.dev/pio"
	"git.edwardh.dev/pio/gpio"
	"git.edwardh.dev/pio/instr"
	"git.edwardh.dev/pio/memory"
	"git.edwardh.dev/pio/simulation"
	"git.edwardh.dev/pio/statemachine"
)

var systemClock float32 = (125 * 1000 * 1000)

var edbusClock float32 = (48 * 1000 * 1000)

var programTX = memory.NewProgramWithOrigin([]uint16{
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
}, 0)

var programTXEntryPoint uint = 6
var programTXWrapFrom uint = 12
var programTXWrapTo uint = 6

var programRX = memory.NewProgramWithOrigin([]uint16{
	instr.EncodeNOP(),
}, 15)

var programRXWrapFrom uint = 0
var programRXWrapTo uint = 0

func setupTX(r *pio.RP2040, pioNumber uint, stateMachineNumber uint, ebdPin uint, ebcPin uint) error {
	if ebdPin+1 != ebcPin {
		return errors.New("ebd+1 != ebc")
	}

	pio, err := r.PIO(pioNumber)
	if err != nil {
		return err
	}

	memoryWriter := pio.MemoryWriter()
	offset, err := memoryWriter.AddProgram(programTX)
	if err != nil {
		return err
	}

	sm, err := pio.StateMachine(stateMachineNumber)
	if err != nil {
		return err
	}

	err = sm.SetWrap(programTXWrapTo+offset, programTXWrapFrom+offset)
	if err != nil {
		return err
	}
	err = sm.SetJumpPin(ebdPin)
	if err != nil {
		return err
	}
	err = sm.SetBaseSidesetPin(ebdPin)
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

	err = pio.SetPinOutput(ebdPin, true)
	if err != nil {
		return err
	}
	err = pio.SetPinOutputEnable(ebdPin, true)
	if err != nil {
		return err
	}
	err = pio.SetPinOutput(ebcPin, true)
	if err != nil {
		return err
	}
	err = pio.SetPinOutputEnable(ebcPin, true)
	if err != nil {
		return err
	}

	var gpioFunction gpio.Function
	switch pioNumber {
	case 0:
		gpioFunction = gpio.FunctionPIO0
	case 1:
		gpioFunction = gpio.FunctionPIO1
	default:
		return errors.New("unknown pio")
	}
	ebdG, err := r.GPIO(ebdPin)
	if err != nil {
		return err
	}
	ebdP, err := r.Pad(ebdPin)
	if err != nil {
		return err
	}
	ebcG, err := r.GPIO(ebcPin)
	if err != nil {
		return err
	}
	ebcP, err := r.Pad(ebcPin)
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

	err = pio.SetPinOutput(ebdPin, false)
	if err != nil {
		return err
	}
	err = pio.SetPinOutput(ebcPin, false)
	if err != nil {
		return err
	}

	sm.Init(offset + programTXEntryPoint)
	sm.ForceInstruction(instr.NewSet(instr.SetDestinationX, 7).Encode() | instr.NewSideset(0b11).Encode(2, false))
	sm.SetEnabled(true)

	return nil
}

func setupRX(r *pio.RP2040, pioNumber uint, stateMachineNumber uint, ebdPin uint, ebcPin uint) error {
	if ebdPin+1 != ebcPin {
		return errors.New("ebd+1 != ebc")
	}

	pio, err := r.PIO(pioNumber)
	if err != nil {
		return err
	}

	memoryWriter := pio.MemoryWriter()
	offset, err := memoryWriter.AddProgram(programRX)
	if err != nil {
		return err
	}

	sm, err := pio.StateMachine(stateMachineNumber)
	if err != nil {
		return err
	}

	err = sm.SetWrap(programRXWrapTo+offset, programRXWrapFrom+offset)
	if err != nil {
		return err
	}
	err = sm.SetBaseInPin(ebdPin)
	if err != nil {
		return err
	}
	err = sm.SetJumpPin(ebcPin)
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

	err := setupTX(r, 0, 0, 6, 7)
	if err != nil {
		return nil, err
	}
	err = setupRX(r, 0, 1, 6, 7)
	if err != nil {
		return nil, err
	}

	return r, nil
}

func setupSimulation() (*simulation.Simulation, *simulation.Net, *simulation.Net, *pio.RP2040, *pio.RP2040, error) {
	s := simulation.New()

	rA, err := newEDBUSRP2040("A")
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	err = s.AddTicker(rA)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	rB, err := newEDBUSRP2040("B")
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	err = s.AddTicker(rB)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}

	ebdNet := simulation.NewNet("EBD")
	ebdConA, err := rA.Connection(6)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	err = ebdNet.Connect(ebdConA)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	ebdConB, err := rB.Connection(6)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	err = ebdNet.Connect(ebdConB)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	err = s.AddNet(ebdNet)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}

	ebcNet := simulation.NewNet("EBC")
	ebcConA, err := rA.Connection(7)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	err = ebcNet.Connect(ebcConA)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	ebcConB, err := rB.Connection(7)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	err = ebcNet.Connect(ebcConB)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	err = s.AddNet(ebcNet)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}

	return s, ebdNet, ebcNet, rA, rB, nil
}
