package main

import (
	"crypto/tls"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"path"
	"strconv"
	"strings"
)

func GetMessageHandlerSetColor(bleLight *BleLight) (handler func(client mqtt.Client, message mqtt.Message)) {
	return func(client mqtt.Client, message mqtt.Message) {
		str := string(message.Payload()[:])
		splitStr := strings.Split(str, ":")
		colorType := splitStr[0]

		if colorType != "rgb" && colorType != "hsb" {
			log.Errorf("color type not supported: '%s'; valid: 'rgb', 'hsb'", colorType)
			return
		}
		if len(splitStr) != 2 {
			log.Errorf("invalid color string format: '%s'", str)
			return
		}

		colorValue := splitStr[1]

		color, err := numberStringToUInt8Slice(colorValue)
		if err != nil {
			log.Errorf("unable to parse color, '%s': %v", str, err)
			return
		}
		if len(color) != 3 {
			log.Errorf("invalid color length: %d", len(color))
			return
		}

		if colorType == "hsb" {
			if err := inPlaceHSBtoRGBConvert(&color); err != nil {
				log.Error("unable to convert HSB to RGB: ", err)
				return
			}
		}

		if err := (*bleLight).SetRGB(color[0], color[1], color[2]); err != nil {
			log.Error("unable to set RGB color: ", err)
			return
		}
	}
}

func GetMessageHandlerSetWhite(bleLight *BleLight) (handler func(client mqtt.Client, message mqtt.Message)) {
	return func(client mqtt.Client, message mqtt.Message) {
		str := string(message.Payload()[:])
		intensity, err := strconv.ParseUint(str, 10, 8)
		if err != nil {
			log.Errorf("unable to parse warm white intensity '%s': %v", str, err)
			return
		}

		if err := (*bleLight).SetWarmWhite(uint8(intensity)); err != nil {
			log.Error("unable to set warm white intensity: ", err)
			return
		}
	}
}

func GetMessageHandlerSetMode(bleLight *BleLight) (handler func(client mqtt.Client, message mqtt.Message)) {
	return func(client mqtt.Client, message mqtt.Message) {
		str := string(message.Payload()[:])
		splitStr := strings.Split(str, ",")
		if len(splitStr) != 2 {
			log.Errorf("invalid number of separators in mode string '%s': %d", str, len(splitStr))
			return
		}

		mode := splitStr[0]
		speed, err := strconv.ParseUint(splitStr[1], 10, 8)
		if err != nil {
			log.Errorf("unable to parse mode speed '%s': %v", str, err)
			return
		}

		if err := (*bleLight).SetMode(mode, uint8(speed)); err != nil {
			log.Error("unable to set mode: ", err)
			return
		}
	}
}

func GetMessageHandlerSetPower(bleLight *BleLight) (handler func(client mqtt.Client, message mqtt.Message)) {
	return func(client mqtt.Client, message mqtt.Message) {
		str := string(message.Payload()[:])

		if str != "on" && str != "off" {
			log.Errorf("invalid power string '%s'; valid: 'on', 'off'", str)
		}

		power := str == "on"
		if err := (*bleLight).SetPower(power); err != nil {
			log.Error("unable to set power: ", err)
			return
		}
	}
}

func StatusChanPublisher(
	mountpoint string,
	client *mqtt.Client,
	statusChan <-chan LightStatus,
	stopChan chan interface{},
	deviceStopChan chan interface{},
) {
	var lastUpdate *map[string]string = nil

	powerTopic := path.Join(mountpoint, "status/power")
	modeTopic := path.Join(mountpoint, "status/mode")
	speedTopic := path.Join(mountpoint, "status/speed")
	rgbTopic := path.Join(mountpoint, "status/rgb")
	hsbTopic := path.Join(mountpoint, "status/hsb")
	whiteTopic := path.Join(mountpoint, "status/white")

Loop:
	for {
		select {
		case status, ok := <-statusChan:
			if !ok {
				break Loop
			}

			update := make(map[string]string)

			if status.Power {
				update[powerTopic] = "on"
			} else {
				update[powerTopic] = "off"
			}
			var mode string
			if status.Mode == "control" {
				if status.WarmWhite {
					mode = "white"
				} else {
					mode = "rgb"
				}

			} else {
				mode = status.Mode
			}
			update[modeTopic] = mode
			update[speedTopic] = string(status.Speed)
			rgbColor := getColorString(status.R, status.G, status.B)
			hsbColor := getColorString(rgbToHSB(status.R, status.G, status.B))
			update[rgbTopic] = rgbColor
			update[hsbTopic] = hsbColor
			update[whiteTopic] = strconv.FormatUint(uint64(status.WarmWhiteIntensity), 10)

			// Remove unchanged values
			if lastUpdate != nil {
				for topic, payload := range *lastUpdate {
					if update[topic] == payload {
						delete(update, topic)
					}
				}
			}

			// Publish all updates
			for topic, payload := range update {
				(*client).Publish(topic, 1, true, payload)
			}

			lastUpdate = &update

			break

		case <-deviceStopChan:
			break Loop
		case <-stopChan:
			break Loop
		}
	}
}

func ConnectClient(config *MQTTConfig) (client mqtt.Client, err error) {
	clientOptions := mqtt.NewClientOptions()
	for _, broker := range config.Servers {
		clientOptions.AddBroker(broker)
	}
	clientOptions.SetAutoReconnect(true)
	if config.ClientID != nil {
		clientOptions.SetClientID(*config.ClientID)
	}
	if config.Username != nil {
		clientOptions.SetUsername(*config.Username)
	}
	if config.Password != nil {
		clientOptions.SetPassword(*config.Password)
	}
	if config.TLS != nil {
		clientOptions.SetTLSConfig(&tls.Config{
			InsecureSkipVerify: config.TLS.InsecureSkipVerify,
		})
	}

	client = mqtt.NewClient(clientOptions)

	if token := client.Connect(); token.Wait() && token.Error() != nil {
		err = token.Error()
	}

	return
}
