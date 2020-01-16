package main

import (
	mqtt "github.com/eclipse/paho.mqtt.golang"
	adapter1 "github.com/muka/go-bluetooth/bluez/profile/adapter"
	"github.com/op/go-logging"
	"os"
	"os/signal"
	"path"
	"syscall"
	"time"
)

const RGBCharUUID string = "0000ffd9-0000-1000-8000-00805f9b34fb"
const NotifyCharUUID string = "0000ffd4-0000-1000-8000-00805f9b34fb"

var log = logging.MustGetLogger("consmart-ble-mqtt")
var format = logging.MustStringFormatter(
	`%{color}%{shortfunc:-15.15s} ▶ %{level:.5s}%{color:reset} %{message}`,
)

func signalHandler(signal chan os.Signal, stopChan chan interface{}) {
	<-signal
	close(stopChan)
}

func getAdapterOrDie(config *Config) (adapter *adapter1.Adapter1) {
	var err error

	if config.Bluetooth != nil && config.Bluetooth.Adapter != nil {
		adapter, err = adapter1.GetAdapter(*config.Bluetooth.Adapter)
		if err != nil {
			log.Fatalf("unable to get adapter '%s': %v\n", *config.Bluetooth.Adapter, err)
		}
	} else {
		adapter, err = adapter1.GetDefaultAdapter()
		if err != nil {
			log.Fatal("unable to retrieve default adapter", err)
		}
	}

	return
}

func requestDeviceUpdates(bleLight *BleLight, deviceStopChan chan interface{}) {
	for {
		select {
		case <-deviceStopChan:
			return
		case <-time.After(1 * time.Second):
			err := (*bleLight).RequestLightStatus()
			if err != nil {
				log.Error("failed to request light status, closing: ", err)
				deviceStopChan <- nil
				return
			}
		}
	}
}

func handleDeviceForever(
	adapter *adapter1.Adapter1,
	addr string,
	deviceConfig DeviceConfig,
	mountpoint string,
	mqttClient mqtt.Client,
	stopChan chan interface{},
) {

	onlineTopic := path.Join(mountpoint, "online")
	colorTopic := path.Join(mountpoint, "control/color")
	modeTopic := path.Join(mountpoint, "control/mode")
	whiteTopic := path.Join(mountpoint, "control/white")
	powerTopic := path.Join(mountpoint, "control/power")

	defer mqttClient.Publish(onlineTopic, 1, true, "false")

	device, err := adapter.GetDeviceByAddress(addr)
	if err != nil {
		log.Errorf("unable to get device '%s': %v", addr, err)
		return
	}
OuterLoop:
	for {
		// Just to be sure
		mqttClient.Unsubscribe(powerTopic, colorTopic, modeTopic, whiteTopic)

		select {
		case <-stopChan:
			goto Disconnect
		default:
			break
		}

		log.Debugf("connecting to '%s'...", addr)

		if ok, err := device.GetConnected(); !ok {
			err := device.Connect()
			if err != nil {
				log.Errorf("unable to connect device '%s', will retry in 5 sec: %v", addr, err)
				time.Sleep(5 * time.Second)
				continue
			}
		} else if err != nil {
			log.Errorf("unable to check whether device '%s' is connected: %v", addr, err)
			return
		}

		log.Debugf("connected to '%s', waiting for services...", addr)

		attempts := 0
		for resolved, err := device.GetServicesResolved(); !resolved; attempts++ {
			if err != nil {
				log.Errorf("unable to check whether services were resolved for '%s': %v", addr, err)
			}
			if attempts >= 10 {
				log.Errorf("unable to check whether services were resolved for '%s' after %d attempts: %v", addr, attempts, err)
				continue OuterLoop
			}
			time.Sleep(1 * time.Second)
		}

		rgbCharUUID := RGBCharUUID
		notifyCharUUID := NotifyCharUUID

		if deviceConfig.RGBCharacteristic != nil {
			rgbCharUUID = *deviceConfig.RGBCharacteristic
		}
		if deviceConfig.NotifyCharacteristic != nil {
			notifyCharUUID = *deviceConfig.NotifyCharacteristic
		}

		rgbChar, err := device.GetCharByUUID(rgbCharUUID)
		if err != nil {
			log.Errorf("unable to retrieve RGB characteristic for '%s': %v", addr, err)
			logCharacteristics(device)
			time.Sleep(1 * time.Second)
			continue
		}
		notifyChar, err := device.GetCharByUUID(notifyCharUUID)
		if err != nil {
			log.Errorf("unable to retrieve notifications characteristic for '%s': %v", addr, err)
			logCharacteristics(device)
			time.Sleep(1 * time.Second)
			continue
		}

		statusChan := make(chan LightStatus)
		bleLight := NewBleLight(rgbChar, notifyChar, statusChan, stopChan)

		mqttClient.Subscribe(powerTopic, 2, GetMessageHandlerSetPower(&bleLight))
		mqttClient.Subscribe(colorTopic, 2, GetMessageHandlerSetColor(&bleLight))
		mqttClient.Subscribe(whiteTopic, 2, GetMessageHandlerSetWhite(&bleLight))
		mqttClient.Subscribe(modeTopic, 2, GetMessageHandlerSetMode(&bleLight))

		deviceStopChan := make(chan interface{})
		go requestDeviceUpdates(&bleLight, deviceStopChan)

		mqttClient.Publish(onlineTopic, 1, true, "true")

		log.Infof("successfully connected to '%s'", addr)

		err = bleLight.ListenNotifications(deviceStopChan)
		if err != nil {
			log.Errorf("error while listening for notifications from '%s': %v", addr, err)
		}

		close(deviceStopChan)
	}

Disconnect:
	mqttClient.Unsubscribe(powerTopic, colorTopic, modeTopic, whiteTopic)

	err = device.Disconnect()
	if err != nil {
		log.Errorf("unable to disconnect device on stop '%s': %v", addr, err)
	}
}

func main() {
	var (
		config  Config
		adapter *adapter1.Adapter1
		err     error
	)
	logging.SetFormatter(format)

	if len(os.Args) != 2 {
		log.Fatalf("usage: %s [config]", os.Args[0])
	}

	config, err = ReadConfig(os.Args[1])
	if err != nil {
		log.Fatal("unable to read config: ", err)
	}

	stopChan := make(chan interface{})

	mqttClient, err := ConnectClient(&config.MQTT)
	if err != nil {
		log.Fatal("unable to connect to MQTT broker: ", err)
	}
	defer mqttClient.Disconnect(0)
	log.Debug("connected to MQTT broker")

	mountpoint := "/"
	if config.MQTT.MountPoint != nil {
		mountpoint = *config.MQTT.MountPoint
	}

	adapter = getAdapterOrDie(&config)
	defer adapter.Close()
	name, _ := adapter.GetName()
	log.Debugf("bluetooth adapter: %s", name)

	for addr, deviceConfig := range config.Devices {
		devMountpoint := path.Join(mountpoint, deviceConfig.MountPoint)
		go handleDeviceForever(adapter, addr, deviceConfig, devMountpoint, mqttClient, stopChan)
	}

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)
	go signalHandler(signalChan, stopChan)

	<-stopChan
	time.Sleep(5 * time.Second)
}
