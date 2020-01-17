package main

import (
	"fmt"
	"github.com/muka/go-bluetooth/bluez/profile/device"
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
