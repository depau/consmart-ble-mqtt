package main

import (
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/muka/go-bluetooth/bluez"
	"github.com/muka/go-bluetooth/bluez/profile/gatt"
)

var LightModes = map[string]uint8{
	"smooth rainbow":       37,
	"pulsating red":        38,
	"pulsating green":      39,
	"pulsating blue":       40,
	"pulsating yellow":     41,
	"pulsating cyan":       42,
	"pulsating magenta":    43,
	"pulsating white":      44,
	"pulsating red/green":  45,
	"pulsating red/blue":   46,
	"pulsating green/blue": 47,
	"rainbow strobe":       48,
	"red strobe":           49,
	"green strobe":         50,
	"blue strobe":          51,
	"yellow strobe":        52,
	"cyan strobe":          53,
	"magenta strobe":       54,
	"white strobe":         55,
	"hard rainbow":         56,
	"pulsating RGB":        97,
	"RGB strobe":           98,
	"hard RGB":             99,
	"control":              65, // Not settable with SetMode
}

var reverseLightModes map[uint8]string = nil

type LightStatus struct {
	R                  uint8
	G                  uint8
	B                  uint8
	Power              bool
	WarmWhite          bool
	WarmWhiteIntensity uint8
	Mode               string
	Speed              uint8
}

type bleLight struct {
	rgbCharacteristic    *gatt.GattCharacteristic1
	notifyCharacteristic *gatt.GattCharacteristic1
	statusChan           chan<- LightStatus
	stopChan             chan interface{}
	propertyChangedChan  chan *bluez.PropertyChanged
}

type BleLight interface {
	SetRGB(r uint8, g uint8, b uint8) (err error)
	SetWarmWhite(intensity uint8) (err error)
	SetPower(powerOn bool) (err error)
	SetMode(mode string, speed uint8) (err error)
	SetModeNumber(mode uint8, speed uint8) (err error)
	RequestLightStatus() (err error)
	ListenNotifications(deviceStopChan chan interface{}) (err error)
}

func NewBleLight(
	rgbCharacteristic *gatt.GattCharacteristic1,
	notifyCharacteristic *gatt.GattCharacteristic1,
	statusChan chan<- LightStatus,
	stopChan chan interface{},
) BleLight {
	return bleLight{
		rgbCharacteristic:    rgbCharacteristic,
		notifyCharacteristic: notifyCharacteristic,
		statusChan:           statusChan,
		stopChan:             stopChan,
	}
}

func makeSetColorPayload(r uint8, g uint8, b uint8, brightness uint8, warmWhite bool) []byte {
	payload := make([]byte, 7)
	payload[0] = 0x56
	payload[1] = r
	payload[2] = g
	payload[3] = b
	payload[4] = brightness
	if warmWhite {
		payload[5] = 0x0F
	} else {
		payload[5] = 0xF0
	}
	payload[6] = 0xAA
	return payload
}

func (light bleLight) SetRGB(r uint8, g uint8, b uint8) error {
	payload := makeSetColorPayload(r, g, b, 0, false)
	return light.rgbCharacteristic.WriteValue(payload, nil)
}

func (light bleLight) SetWarmWhite(intensity uint8) error {
	payload := makeSetColorPayload(0, 0, 0, intensity, true)
	return light.rgbCharacteristic.WriteValue(payload, nil)
}

func (light bleLight) SetPower(powerOn bool) error {
	payload := make([]byte, 3)
	payload[0] = 0xCC
	if powerOn {
		payload[1] = 0x23
	} else {
		payload[1] = 0x24
	}
	payload[2] = 0x33
	return light.rgbCharacteristic.WriteValue(payload, nil)
}

func (light bleLight) SetMode(mode string, speed uint8) error {
	if val, ok := LightModes[mode]; !ok {
		return errors.New(fmt.Sprintf("mode '%s' is not valid", mode))
	} else {
		return light.SetModeNumber(val, speed)
	}
}

func (light bleLight) SetModeNumber(mode uint8, speed uint8) error {
	if mode == 65 {
		return errors.New("RGB control mode can't be set with SetMode")
	} else if speed > 31 || speed < 1 {
		return errors.New("speed must be between 1 and 31 (and is inversely proportional)")
	} else {
		payload := make([]byte, 4)
		payload[0] = 0xBB
		payload[1] = mode
		payload[2] = speed
		payload[3] = 0x44
		return light.rgbCharacteristic.WriteValue(payload, nil)
	}
}

func (light bleLight) RequestLightStatus() error {
	payload := []byte{0xEF, 0x01, 0x77}
	return light.rgbCharacteristic.WriteValue(payload, nil)
}

func (light bleLight) propertyChangedWatcher(deviceStopChan chan interface{}) {
Loop:
	for {
		select {
		case prop, ok := <-light.propertyChangedChan:
			if !ok || prop == nil {
				break Loop
			}
			if prop.Interface != "org.bluez.GattCharacteristic1" || prop.Name != "Value" {
				break
			}
			value := prop.Value.([]byte)

			if value[0] != 0x66 || len(value) < 10 {
				hexdump := hex.Dump(value)
				log.Debug("unrecognized notification value, don't know how to handle: ", hexdump)
				continue Loop
			}

			lightStatus := LightStatus{}
			lightStatus.Power = value[2] == 0x23
			lightStatus.Mode = reverseLightModes[value[3]]
			lightStatus.Speed = value[5]
			lightStatus.R = value[6]
			lightStatus.G = value[7]
			lightStatus.B = value[8]
			lightStatus.WarmWhiteIntensity = value[9]
			lightStatus.WarmWhite = lightStatus.WarmWhiteIntensity != 0 || lightStatus.Mode != "control"

			light.statusChan <- lightStatus

		case <-deviceStopChan:
			break Loop
		case <-light.stopChan:
			break Loop
		}
	}
	log.Debug("stopped listening for notifications")
}

func populateReverseLightModes() {
	reverseLightModes = make(map[uint8]string)
	for key, val := range LightModes {
		reverseLightModes[val] = key
	}
}

func (light bleLight) ListenNotifications(deviceStopChan chan interface{}) (err error) {
	if reverseLightModes == nil {
		populateReverseLightModes()
	}

	if light.propertyChangedChan, err = light.notifyCharacteristic.WatchProperties(); err != nil {
		return
	}

	if err = light.notifyCharacteristic.StartNotify(); err != nil {
		return
	}

	light.propertyChangedWatcher(deviceStopChan)

	err = light.notifyCharacteristic.StopNotify()
	return
}
