package main

import (
	"errors"

	"git.edwardh.dev/pio"
	"git.edwardh.dev/pio/gpio"
	"git.edwardh.dev/pio/memory"
	"git.edwardh.dev/pio/simulation"
	"git.edwardh.dev/pio/statemachine"
)

var systemClock float32 = (125 * 1000 * 1000)

var edbusClock float32 = (48 * 1000 * 1000)

var programTX = memory.NewProgramWithOrigin([]uint16{
	0xc020, //  0: irq    wait 0
	//     .wrap_target
	0x80a0, //  1: pull   block
	0x2047, //  2: wait   0 irq, 7
	0xf780, //  3: set    pindirs, 0      side 0 [7]
	0xe027, //  4: set    x, 7
	0x80e0, //  5: pull   ifempty block
	0x6041, //  6: out    y, 1
	0x006b, //  7: jmp    !y, 11
	0xfa81, //  8: set    pindirs, 1      side 1 [2]
	0x00cc, //  9: jmp    pin, 12
	0x0000, // 10: jmp    0
	0xfb80, // 11: set    pindirs, 0      side 1 [3]
	0x10e6, // 12: jmp    !osre, 6        side 0
	0x0045, // 13: jmp    x--, 5
	0xff81, // 14: set    pindirs, 1      side 1 [7]
	0xc021, // 15: irq    wait 1
	//     .wrap
}, 0)

var programTXEntryPoint uint = 1
var programTXWrapFrom uint = 15
var programTXWrapTo uint = 1

var programRX = memory.NewProgramWithOrigin([]uint16{
	//     .wrap_target
	0xc047, //  0: irq    clear 7
	0x2021, //  1: wait   0 pin, 1
	0xc007, //  2: irq    nowait 7
	0xe127, //  3: set    x, 7                   [1]
	0x01cb, //  4: jmp    pin, 11                [1]
	0xe05f, //  5: set    y, 31
	0x22a1, //  6: wait   1 pin, 1               [2]
	0x4001, //  7: in     pins, 1
	0x0086, //  8: jmp    y--, 6
	0x8000, //  9: push   noblock
	0x0045, // 10: jmp    x--, 5
	//     .wrap
	0x23a1, // 11: wait   1 pin, 1               [3]
	0x00c0, // 12: jmp    pin, 0
	0x000b, // 13: jmp    11
}, 16)

var programRXWrapFrom uint = 10
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
	err = sm.SetBaseSetPin(ebdPin)
	if err != nil {
		return err
	}
	err = sm.SetPinCountSet(1)
	if err != nil {
		return err
	}
	err = sm.SetJumpPin(ebdPin)
	if err != nil {
		return err
	}
	err = sm.SetBaseSidesetPin(ebcPin)
	if err != nil {
		return err
	}
	err = sm.SetSideset(2, true, true)
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
