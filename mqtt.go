package main

import (
	"crypto/tls"
	"fmt"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"path"
	"strconv"
	"strings"
)

func GetMessageHandlerSetColor(bleLight *BleLight) (handler func(client mqtt.Client, message mqtt.Message)) {
	return func(client mqtt.Client, message mqtt.Message) {
		colorValue := string(message.Payload()[:])
		color, err := numberStringToUInt8Slice(colorValue)
		if err != nil {
			log.Errorf("unable to parse color, '%s': %v", colorValue, err)
			return
		}
		if len(color) != 3 {
			log.Errorf("invalid color length: %d", len(color))
			return
		}
		r := color[0]
		g := color[1]
		b := color[2]

		// Simulate simple power control to be nice to Google Assistant
		if r == g && g == b && b == r && r == 0 {
			if err := (*bleLight).SetPower(false); err != nil {
				log.Error("unable to turn off light: ", err)
			}
			return
		} else {
			if err := (*bleLight).SetPower(true); err != nil {
				log.Error("unable to turn on light: ", err)
				// ignore error, light might be already on so we can as well try to set the other values
			}
		}

		// Simulate simple white control
		if r == g && g == b && b == r {
			if err := (*bleLight).SetWarmWhite(r); err != nil {
				log.Error("unable to set white intensity: ", err)
			}
			return
		}

		if err := (*bleLight).SetRGB(r, g, b); err != nil {
			log.Error("unable to set RGB color: ", err)
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

		if str != "off" && str != "on" {
			log.Error("invalid power control string: ", str)
		}

		if err := (*bleLight).SetPower(str == "on"); err != nil {
			log.Error("unable to set light power: ", err)
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

	modeTopic := path.Join(mountpoint, "status/mode")
	rgbTopic := path.Join(mountpoint, "status/color")
	powerTopic := path.Join(mountpoint, "status/power")

Loop:
	for {
		select {
		case status, ok := <-statusChan:
			if !ok {
				break Loop
			}

			update := make(map[string]string)

			var mode string
			if status.Mode == "control" {
				if status.WarmWhite {
					mode = "white"
				} else {
					mode = "rgb"
				}
			} else {
				mode = fmt.Sprintf("%s,%d", status.Mode, status.Speed)
			}
			update[modeTopic] = mode
			update[rgbTopic] = getColorString(status.R, status.G, status.B)
			if status.Power {
				update[powerTopic] = "on"
			} else {
				update[powerTopic] = "off"
			}

			// Publish only changed values
			for topic, payload := range update {
				if lastUpdate != nil {
					if oldPayload := (*lastUpdate)[topic]; oldPayload == payload {
						continue
					}
				}
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
