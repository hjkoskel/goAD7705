package goAD7705_test

/*
Example how to use AD7705.
Test on raspberry or use this code as example and cross compile
*/

import (
	"fmt"

	"github.com/davecheney/gpio/rpi"
	"github.com/hjkoskel/goAD7705"
)

func ExampleReading() {
	// ad, err := goAD7705.InitAD7705("/dev/spidev0.0", -1)   //Or use -1 as ready pin then software reads ready status from i2c

	ad, err := goAD7705.InitAD7705("/dev/spidev0.0", rpi.GPIO25) //Use hardware gpio as ready pin. By default, sudo is needed for gpio

	if err != nil {
		fmt.Printf("Initialization error %#v\n", err)
		return
	}
	ad.SetBuffered(goAD7705.CH_AIN1, true)
	ad.SetUnipolar(goAD7705.CH_AIN1)

	for {
		waitErr := ad.WaitDataReady(goAD7705.CH_AIN1)
		if waitErr != nil {
			fmt.Printf("WAIT ERROR %#v\n", waitErr)
			return
		}

		//value, adcErr := ad.ReadVoltage(goAD7705.CH_AIN1)  TODO: bipolar/unipolar not tested/developed yet
		value, adcErr := ad.ReadData(goAD7705.CH_AIN1)
		if adcErr == nil {
			fmt.Printf("ADC value=%.2v\n", value)
		} else {
			fmt.Printf("READ ERROR = %#v\n", adcErr)
			return
		}
	}
}
