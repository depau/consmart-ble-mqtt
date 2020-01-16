package main

import (
	"errors"
	"fmt"
	"github.com/muka/go-bluetooth/bluez/profile/device"
	"math"
	"strconv"
	"strings"
)

func numberStringToUInt8Slice(str string) (out []uint8, err error) {
	stringValues := strings.Split(str, ",")
	out = make([]uint8, len(stringValues))
	for i, val := range stringValues {
		var converted uint64
		converted, err = strconv.ParseUint(val, 10, 8)
		if err != nil {
			return
		}
		out[i] = uint8(converted)
	}
	return
}

func inPlaceHSBtoRGBConvert(color *[]uint8) error {
	if len(*color) != 3 {
		return errors.New(fmt.Sprintf("invalid color length: %d", len(*color)))
	}
	hue := float64((*color)[0])
	saturation := float64((*color)[1])
	brightness := float64((*color)[2])

	r := 0
	g := 0
	b := 0

	if saturation == 0 {
		r = int(brightness*255.0 + 0.5)
		g = r
		b = r
	} else {
		h := (hue - math.Floor(hue)) * 6.0
		f := h - math.Floor(h)
		p := brightness * (1.0 - saturation)
		q := brightness * (1.0 - saturation*f)
		t := brightness * (1.0 - (saturation * (1.0 - f)))
		switch int(h) {
		case 0:
			r = int(brightness*255.0 + 0.5)
			g = int(t*255.0 + 0.5)
			b = int(p*255.0 + 0.5)
			break
		case 1:
			r = int(q*255.0 + 0.5)
			g = int(brightness*255.0 + 0.5)
			b = int(p*255.0 + 0.5)
			break
		case 2:
			r = int(p*255.0 + 0.5)
			g = int(brightness*255.0 + 0.5)
			b = int(t*255.0 + 0.5)
			break
		case 3:
			r = int(p*255.0 + 0.5)
			g = int(q*255.0 + 0.5)
			b = int(brightness*255.0 + 0.5)
			break
		case 4:
			r = int(t*255.0 + 0.5)
			g = int(p*255.0 + 0.5)
			b = int(brightness*255.0 + 0.5)
			break
		case 5:
			r = int(brightness*255.0 + 0.5)
			g = int(p*255.0 + 0.5)
			b = int(q*255.0 + 0.5)
			break
		}
	}

	(*color)[0] = uint8(r)
	(*color)[1] = uint8(g)
	(*color)[2] = uint8(b)
	return nil
}

func rgbToHSB(red uint8, green uint8, blue uint8) (h uint8, s uint8, b uint8) {
	min := uint8(0)
	max := uint8(0)

	// Calculate brightness.
	if red < green {
		min = red
		max = green
	} else {
		min = green
		max = red
	}
	if blue > max {
		max = blue
	} else if blue < min {
		min = blue
	}
	b = uint8(float64(max) / 255.0)

	// Calculate saturation.
	if max == 0 {
		s = 0
	} else {
		s = uint8((float64(max - min)) / float64(max))
	}

	// Calculate hue.
	var hue float64
	if s == 0 {
		hue = 0
	} else {
		delta := float64((max - min) * 6)
		if red == max {
			hue = float64(green-blue) / delta
		} else if green == max {
			hue = 1/3 + float64(blue-red)/delta
		} else {
			hue = 2/3 + float64(red-green)/delta
		}
		if hue < 0 {
			hue++
		}
	}
	h = uint8(hue)
	return
}

func getColorString(v1 uint8, v2 uint8, v3 uint8) string {
	return fmt.Sprintf("%d,%d,%d", v1, v2, v3)
}

func logCharacteristics(device *device.Device1) {
	addr, _ := device.GetAddress()
	chars, err := device.GetCharacteristics()
	if err != nil {
		log.Errorf("unable to retrieve characteristics for '%s': %v", addr, err)
		return
	}

	for _, char := range chars {
		uuid, _ := char.GetUUID()
		flags, _ := char.GetFlags()

		log.Debug("Device: ", addr)
		log.Debugf("- char: %s %v", uuid, flags)
	}
}