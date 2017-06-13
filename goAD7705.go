/*
Sneak peak code release for my blog.
Not tested with analog side hardware, so some issues might be with bipolar voltage calculation etc..
I will release next version soon.
At least this is good example how to use SPI.
*/

package goAD7705

import (
	"fmt"
	"time"

	"github.com/davecheney/gpio"
	"golang.org/x/exp/io/spi"
)

type ChSetup struct {
	//For setup register
	Gain     Gainsel `json:"gain"`
	Unipolar bool    `json:"unipolar"`
	Buffered bool    `json:"buffered"`
	Filtsync bool    `json:"filtsync"`
	//For clock register Clk and clkdiv are set on setup... only filter is adjusted
	Odr ODR `json:"odr"`
}

//Parses setup from associative array

func (p *ChSetup) ParseSetup(settings map[string]float64) {
	if f, ok := settings["gain"]; ok {
		p.Gain = Gainsel(f)
	}
	if f, ok := settings["bipolar"]; ok {
		p.Unipolar = (f < 1)
	}
	if f, ok := settings["unipolar"]; ok {
		p.Unipolar = (0 < f)
	}

	if f, ok := settings["buffered"]; ok {
		p.Buffered = (0 < f)
	}

	if f, ok := settings["filtsync"]; ok {
		p.Filtsync = (0 < f)
	}

	if f, ok := settings["odr"]; ok {
		p.Odr = ODR(f)
	}

}

type AD7705 struct {
	spiConn  *spi.Device
	clockDiv bool //at startup only
	xtal     bool //CLK bit at startup
	readyPin gpio.Pin
}

const (
	REG_COMMUNICATION = 0
	REG_SETUP         = 1
	REG_CLOCK         = 2
	REG_DATA          = 3
	REG_TEST          = 4
	REG_NOP           = 5
	REG_OFFSET        = 6
	REG_GAIN          = 7
)

type RegSel byte

func regCodeSetup(mode Opmode, gain Gainsel, unipolar bool, buffered bool, fsync bool) byte {
	result := (byte(mode) << 6) | (byte(gain) << 3)
	if unipolar {
		result |= (1 << 2)
	}
	if buffered {
		result |= (1 << 1)
	}
	if fsync {
		result |= 1
	}
	return result
}

func regCodeClock(clkDis bool, clkDiv bool, clk bool, odr ODR) byte {
	result := byte(odr)
	if clkDis {
		result |= (1 << 4)
	}
	if clkDiv {
		result |= (1 << 3)
	}
	if clk {
		result |= (1 << 2)
	}
	return result
}

const (
	CH_AIN1        = 0
	CH_AIN2        = 1
	CH_AIN1_COMMON = 2
	CH_AIN2_COMMON = 3
)

type Chsel byte

const (
	OPMODE_NORMAL  = 0
	OPMODE_SELFCAL = 1
	OPMODE_ZEROCAL = 2
	OPMODE_FULLCAL = 3
)

type Opmode byte

const (
	GAINSEL_1   = 0
	GAINSEL_2   = 1
	GAINSEL_4   = 2
	GAINSEL_8   = 3
	GAINSEL_16  = 4
	GAINSEL_32  = 5
	GAINSEL_64  = 6
	GAINSEL_128 = 7
)

type Gainsel byte

func GainInNumber(g Gainsel) float64 {
	return []float64{1, 2, 4, 8, 16, 32, 64, 128}[g]
}

func PickGain(gain float64) Gainsel {
	if 128 <= gain {
		return GAINSEL_128
	}
	if 64 <= gain {
		return GAINSEL_64
	}
	if 32 <= gain {
		return GAINSEL_32
	}
	if 16 <= gain {
		return GAINSEL_16
	}
	if 8 <= gain {
		return GAINSEL_8
	}
	if 4 <= gain {
		return GAINSEL_4
	}
	if 2 <= gain {
		return GAINSEL_2
	}
	return GAINSEL_1
}

const (
	ODRXTAL_20  = 0
	ODRXTAL_25  = 1
	ODRXTAL_100 = 2
	ODRXTAL_200 = 3
	ODROSC_50   = 0
	ODROSC_60   = 1
	ODROSC_250  = 2
	ODROSC_500  = 3
)

type ODR byte

func (p *AD7705) Set(ch Chsel, setup ChSetup) error {
	_, err := p.WriteRegister(ch, REG_CLOCK, regCodeClock(false, p.clockDiv, p.xtal, setup.Odr))
	if err != nil {
		return err
	}
	_, err = p.WriteRegister(ch, REG_SETUP, regCodeSetup(OPMODE_NORMAL, setup.Gain, setup.Unipolar, setup.Buffered, setup.Filtsync))
	return err
}

//Clockdiv and xtal settings depend on pcb.. not adjustable during run
func InitAD7705(spidevice string, readypin int, xtal bool, clkdiv bool, setup ChSetup) (AD7705, error) {
	fmt.Printf("Initializing AD7705\n")
	result := AD7705{}

	result.clockDiv = clkdiv
	result.xtal = xtal

	if 0 < readypin {
		pin, err := gpio.OpenPin(readypin, gpio.ModeInput)
		result.readyPin = pin
		if err != nil {
			fmt.Printf("Failed opening pin! %#v\n", err)
			return result, err
		}
	}
	fmt.Printf("Going to open\n")
	dev, err := spi.Open(&spi.Devfs{
		Dev:      spidevice, // "/dev/spidev0.0",
		Mode:     spi.Mode3,
		MaxSpeed: 50000,
	})
	if err != nil {
		return result, err
	}
	if err = dev.SetDelay(200 * time.Millisecond); err != nil {
		return result, err
	}
	if err = dev.SetBitsPerWord(8); err != nil {
		return result, err
	}
	if err = dev.SetCSChange(true); err != nil {
		return result, err
	}

	result.spiConn = dev

	fmt.Printf("Going to reset\n")
	if err = result.BusReset(); err != nil {
		return result, err
	}
	fmt.Printf("Reset done Going to set channels\n")

	if err = result.Set(CH_AIN1, setup); err != nil {
		return result, err
	}
	if err = result.Set(CH_AIN2, setup); err != nil {
		return result, err
	}
	return result, nil
}

func (p *AD7705) BusReset() error {
	tx := []byte{0xFF}
	rx := []byte{0}
	for i := 0; i < 100; i++ {
		tx[0] = 0xFF
		if err := p.spiConn.Tx(tx, rx); err != nil {
			return err
		}
	}
	return nil
}

func (p *AD7705) WriteRegister(chSelection Chsel, register RegSel, data byte) (byte, error) {
	tx := []byte{byte(register<<4) | byte(chSelection)}
	rx := make([]byte, len(tx))
	if err := p.spiConn.Tx(tx, rx); err != nil {
		return 0, err
	}
	tx[0] = data
	if err := p.spiConn.Tx(tx, rx); err != nil {
		return 0, err
	}
	return rx[0], nil
}

func (p *AD7705) WriteRegister24(chSelection Chsel, register RegSel, data uint32) (uint32, error) {
	tx := []byte{byte(register<<4) | byte(chSelection)}
	rx := make([]byte, len(tx))
	if err := p.spiConn.Tx(tx, rx); err != nil {
		return 0, err
	}
	tx = []byte{byte(data >> 16), byte((data >> 8)) & byte(0xFF), byte(data & 0xFF)}
	if err := p.spiConn.Tx(tx, rx); err != nil {
		return 0, err
	}
	return (uint32(rx[2]) << 16) | (uint32(rx[1]) << 8) | (uint32(rx[0])), nil
}

func (p *AD7705) readRegister(chSelection Chsel, register RegSel) (byte, error) {
	tx := []byte{byte(register<<4) | (1 << 3) | byte(chSelection)}
	rx := make([]byte, len(tx))
	if err := p.spiConn.Tx(tx, rx); err != nil {
		return rx[0], err
	}
	tx[0] = 0
	if err := p.spiConn.Tx(tx, rx); err != nil {
		return rx[0], err
	}
	return rx[0], nil
}

func (p *AD7705) readRegister24(chSelection Chsel, register RegSel) (uint32, error) {
	tx := []byte{byte(register<<4) | (1 << 3) | byte(chSelection)}
	rx := make([]byte, len(tx))
	if err := p.spiConn.Tx(tx, rx); err != nil {
		return 0, err
	}
	tx[0] = 0
	if err := p.spiConn.Tx(tx, rx); err != nil {
		return 0, err
	}
	result := uint32(rx[0]) << 16
	tx[0] = 0
	if err := p.spiConn.Tx(tx, rx); err != nil {
		return 0, err
	}
	result |= uint32(rx[0]) << 8
	tx[0] = 0
	if err := p.spiConn.Tx(tx, rx); err != nil {
		return 0, err
	}
	result |= uint32(rx[0])
	return result, nil
}

func (p *AD7705) ReadData(chSelection Chsel) (uint16, error) {
	tx0 := []byte{byte(REG_DATA<<4) | (1 << 3) | byte(chSelection)}
	rx0 := make([]byte, len(tx0))
	if err := p.spiConn.Tx(tx0, rx0); err != nil {
		return 0, err
	}

	tx := []byte{0}
	rx := []byte{0}
	if err := p.spiConn.Tx(tx, rx); err != nil {
		return 0, err
	}
	hi := rx[0]

	tx = []byte{0}
	rx = []byte{0}
	if err := p.spiConn.Tx(tx, rx); err != nil {
		return 0, err
	}
	lo := rx[0]

	fmt.Printf("DATA hi=%X lo=%X\n", hi, lo)
	return (uint16(hi) << 8) | uint16(lo), nil
}

func (p *AD7705) WaitDataReady(chSelection Chsel) error {
	if p.readyPin != nil {
		for p.readyPin.Get() {
		}
		return nil
	}
	for {
		regValue, err := p.readRegister(chSelection, REG_COMMUNICATION)
		//fmt.Printf("Wait result=%v\n", regValue)
		if err != nil {
			return err
		}
		if regValue&0x80 == 0 {
			return nil
		}
	}
}

func PickRightGainFor(voltage float64) Gainsel {
	vMaxInternal := 2.5
	if voltage/vMaxInternal < (1.0 / 128.0) {
		return GAINSEL_128
	}
	if voltage/vMaxInternal < (1.0 / 64.0) {
		return GAINSEL_64
	}
	if voltage/vMaxInternal < (1.0 / 32.0) {
		return GAINSEL_32
	}
	if voltage/vMaxInternal < (1.0 / 16.0) {
		return GAINSEL_16
	}
	if voltage/vMaxInternal < (1.0 / 8.0) {
		return GAINSEL_8
	}
	if voltage/vMaxInternal < (1.0 / 4.0) {
		return GAINSEL_4
	}
	if voltage/vMaxInternal < (1.0 / 2.0) {
		return GAINSEL_2
	}
	return GAINSEL_1
}

/*
Calibrations.

Possible calibration operations
1)Self-calibration:
	internal zero and full scale

2)Zero scale system calibration
3)Full scale system calibration

*/

func (p *AD7705) SelfCal(chSelection Chsel, setup ChSetup, mode Opmode) error {
	if _, err := p.WriteRegister(chSelection, REG_SETUP, regCodeSetup(mode, setup.Gain, setup.Unipolar, setup.Buffered, setup.Filtsync)); err != nil {
		return err
	}
	time.Sleep(1000 * time.Millisecond) //Testing
	return p.WaitDataReady(chSelection)
}
