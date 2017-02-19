/*
Sneak peak code release for my blog.
Not tested with analog side hardware, so some issues might be with bipolar voltage calculation etc..
I will release next version soon.
At least this is good example how to use SPI.
*/

package goAD7705

import (
	"fmt"

	"github.com/davecheney/gpio"

	"golang.org/x/exp/io/spi"
)

type AD7705 struct {
	spiConn *spi.Device
	//selectRegValue byte //Without

	//clock register status
	masterClockDisable bool
	clockDiv           bool
	odr                [4]ODR //4 possible channel settings

	readyPin gpio.Pin

	//setup register
	mode     [4]Opmode
	gain     [4]Gainsel
	unipolar [4]bool
	buffered [4]bool
	filtsync [4]bool
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

/*
Pass negative number and AD wont wait readypin
*/
func InitAD7705(spidevice string, readypin int) (AD7705, error) {

	result := AD7705{
		masterClockDisable: false,
		clockDiv:           true,
		odr:                [4]ODR{ODR_50, ODR_50, ODR_50, ODR_50},
		mode:               [4]Opmode{OPMODE_SELFCAL, OPMODE_SELFCAL, OPMODE_SELFCAL, OPMODE_SELFCAL},
		gain:               [4]Gainsel{GAINSEL_1, GAINSEL_1, GAINSEL_1, GAINSEL_1},
		unipolar:           [4]bool{false, false, false, false},
		buffered:           [4]bool{false, false, false, false},
		filtsync:           [4]bool{false, false, false, false},
	}

	if 0 < readypin {

		pin, err := gpio.OpenPin(readypin, gpio.ModeInput)
		result.readyPin = pin
		if err != nil {
			fmt.Printf("Failed opening pin! %#v\n", err)
			return result, err
		}
	}

	/*
		d := &spi.Devfs{
			Dev:      spidevice, // "/dev/spidev0.0",
			Mode:     spi.Mode3,
			MaxSpeed: 50000,
		}

		dev, err := d.Open()
		if err != nil {
			fmt.Printf("Cannot read device %#v\n", err)
			return result, err
		}
	*/

	dev, err := spi.Open(&spi.Devfs{
		Dev:      spidevice, // "/dev/spidev0.0",
		Mode:     spi.Mode3,
		MaxSpeed: 50000,
	})
	if err != nil {
		return result, err
	}
	dev.SetBitsPerWord(8) //IMPORTANT
	dev.SetCSChange(true)

	result.spiConn = dev
	//result.spiConn.SetDelay(100 * time.Millisecond)

	//result.busReset()

	//setup register
	result.updateRegClock(CH_AIN1)
	result.updateRegSetup(CH_AIN1)
	result.WaitDataReady(CH_AIN1)
	//time.Sleep(time.Second * 2)

	result.updateRegClock(CH_AIN2)
	result.updateRegSetup(CH_AIN2)
	result.WaitDataReady(CH_AIN2)

	return result, nil
}

func (p *AD7705) SetMasterClockDisable(chSelection Chsel, masterClockDisable bool) error {
	p.masterClockDisable = masterClockDisable
	return p.updateRegClock(chSelection)
}

/*
masterClockDisable bool
clockDiv bool
odr ODR
*/

func (p *AD7705) SetClockdiv(chSelection Chsel, clkdiv bool) error {
	p.clockDiv = clkdiv
	return p.updateRegClock(chSelection)
}

const (
	ODR_20  = 0
	ODR_25  = 1
	ODR_100 = 2
	ODR_200 = 3
	ODR_50  = 4
	ODR_60  = 5
	ODR_250 = 6
	ODR_500 = 7
)

type ODR byte

func (p *AD7705) SetOutputDataRate(chSelection Chsel, rate ODR) error {
	p.odr[chSelection] = rate
	return p.updateRegClock(chSelection)
}

/*
	mode [4]Opmode
	gain [4]Chsel
	unipolar bool
	buffered bool
*/

func (p *AD7705) SetOpMode(chSelection Chsel, mode Opmode) error {
	p.mode[chSelection] = mode
	return p.updateRegSetup(chSelection)
}

func (p *AD7705) SetGain(chSelection Chsel, gain Gainsel) error {
	p.gain[chSelection] = gain
	return p.updateRegSetup(chSelection)
}

func (p *AD7705) SetUnipolar(chSelection Chsel) error {
	p.unipolar[chSelection] = true
	return p.updateRegSetup(chSelection)
}

func (p *AD7705) SetBipolar(chSelection Chsel) error {
	p.unipolar[chSelection] = true
	return p.updateRegSetup(chSelection)
}

func (p *AD7705) SetBuffered(chSelection Chsel, buffered bool) error {
	p.buffered[chSelection] = buffered
	return p.updateRegSetup(chSelection)
}

func (p *AD7705) SetFiltSync(chSelection Chsel, filtsync bool) error {
	p.filtsync[chSelection] = filtsync
	return p.updateRegSetup(chSelection)
}

func (p *AD7705) ReadData(chSelection Chsel) (uint16, error) {
	tx := []byte{byte(REG_DATA<<4) | byte(chSelection) | (1 << 3), 0, 0}
	rx := make([]byte, len(tx))
	err := p.spiConn.Tx(tx, rx)
	//fmt.Printf("read data tx=%#v rx=%#v\n", tx, rx)
	if err != nil {
		return 0, err
	}
	return uint16(rx[1]) + uint16(rx[2])<<8, nil
}

//Uses registers if ready line is not available
func (p *AD7705) DataReady(chSelection Chsel) (bool, error) {
	if p.readyPin != nil {
		pinvalue := p.readyPin.Get()
		//fmt.Printf("Ready pin is %v\n", pinvalue)
		return !pinvalue, nil //Down is ready?
	}

	tx := []byte{byte(REG_COMMUNICATION<<4) | byte(chSelection) | (1 << 3), 0}
	rx := make([]byte, len(tx))
	err := p.spiConn.Tx(tx, rx)
	if err != nil {
		return false, err
	}
	//fmt.Printf("Ready returns %#v\n", rx)
	return rx[1]&0x80 == 0, err
}

func (p *AD7705) WaitDataReady(chSelection Chsel) error {
	for {
		ready, err := p.DataReady(chSelection)
		if err != nil {
			return err
		}
		if ready {
			return nil
		}
	}
}

func (p *AD7705) ReadVoltage(chSelection Chsel) (float64, error) {
	r, err := p.ReadData(chSelection)
	if err != nil {
		return 0, err
	}
	gain := float64(byte(1 << byte(p.gain[chSelection])))
	//fmt.Printf("ch %v gain is %v hex=%X value=%v\n", chSelection, gain, r, r)
	volts := float64(r) * 5 / (float64(0xFFFF) * gain)

	//For ADC test not calibrated
	/*	volts -= 0.820325016
		volts /= 0.8632
	*/
	return volts, nil
}

//Internal use, copies struct status to actual hardware
func (p *AD7705) updateRegClock(chSelection Chsel) error {
	clockbyte := byte(p.odr[chSelection])
	if p.clockDiv {
		clockbyte |= (1 << 3)
	}
	if p.masterClockDisable {
		clockbyte |= (1 << 4)
	}
	tx := []byte{byte(REG_CLOCK<<4) | byte(chSelection), clockbyte}
	rx := make([]byte, len(tx))
	err := p.spiConn.Tx(tx, rx)
	fmt.Printf("Update reg clock tx=%#X rx=%#v\n", tx, rx)
	return err
}

//Internal use, copies struct status to actual hardware
func (p *AD7705) updateRegSetup(chSelection Chsel) error {
	setupbyte := (byte(p.mode[chSelection]) << 6) | (byte(p.gain[chSelection]) << 3)
	if p.unipolar[chSelection] {
		setupbyte |= (1 << 2)
	}
	if p.buffered[chSelection] {
		setupbyte |= (1 << 1)
	}
	if p.filtsync[chSelection] {
		setupbyte |= 1
	}
	tx := []byte{byte(REG_SETUP<<4) | byte(chSelection), setupbyte}
	rx := make([]byte, len(tx))
	err := p.spiConn.Tx(tx, rx)
	//fmt.Printf("Update reg setup tx=%#X rx=%#v\n", tx, rx)

	return err
}

func (p *AD7705) busReset() error {
	tx := make([]byte, 10)

	for i := 0; i < 10; i++ {
		tx[i] = 0xFF

	}
	for i := 0; i < 10; i++ {
		rx := make([]byte, len(tx))
		err := p.spiConn.Tx(tx, rx)
		if err != nil {
			return err
		}
	}
	return nil
}
