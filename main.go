package main

import (
	mqtt "github.com/eclipse/paho.mqtt.golang"
	adapter1 "github.com/muka/go-bluetooth/bluez/profile/adapter"
	device2 "github.com/muka/go-bluetooth/bluez/profile/device"
	"github.com/op/go-logging"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"runtime"
	"strings"
	"syscall"
	"time"
)

const RGBCharUUID string = "0000ffd9-0000-1000-8000-00805f9b34fb"
const NotifyCharUUID string = "0000ffd4-0000-1000-8000-00805f9b34fb"

var log = logging.MustGetLogger("consmart-ble-mqtt")
var format = logging.MustStringFormatter(
	`%{color}%{shortfunc:-15.15s} ▶ %{level:.5s}%{color:reset} %{message}`,
)

func signalHandler(signal chan os.Signal, stopRope StopRope) {
	for {
		sig := <-signal
		if sig == syscall.SIGQUIT {
			buf := make([]byte, 1<<20)
			stacklen := runtime.Stack(buf, true)
			log.Debugf("=== received SIGQUIT ===\n*** goroutine dump...\n%s\n*** end", buf[:stacklen])
		} else {
			stopRope.Cut()
			return
		}
	}

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

func requestDeviceUpdates(bleLight *BleLight, stopRope StopRope, bluetoothResetChan chan bool) {
	if err := stopRope.Hold(); err != nil {
		return
	}
	defer stopRope.Release()

	for {
		select {
		case <-stopRope.WaitCut():
			return
		case <-time.After(1 * time.Second):
			err := (*bleLight).RequestLightStatus()
			if err != nil {
				if strings.Contains(err.Error(), "Input/output error") {
					bluetoothResetChan <- true
					log.Error("failed to request light status, bluetooth needs reset: ", err)
				} else {
					log.Error("failed to request light status, closing: ", err)
				}
				stopRope.Cut()
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
	stopRope StopRope,
	bluetoothResetChan chan bool,
) {
	if err := stopRope.Hold(); err != nil {
		return
	}
	defer stopRope.Release()

	onlineTopic := path.Join(mountpoint, "online")
	colorTopic := path.Join(mountpoint, "control/color")
	modeTopic := path.Join(mountpoint, "control/mode")
	powerTopic := path.Join(mountpoint, "control/power")

	defer mqttClient.Publish(onlineTopic, 1, true, "false")

OuterLoop:
	for {
		if stopRope.IsCut() {
			break OuterLoop
		}

		device, err := adapter.GetDeviceByAddress(addr)
		if err != nil {
			log.Errorf("unable to get device '%s': %v", addr, err)
			return
		}

		log.Debugf("connecting to '%s'...", addr)

		if ok, err := device.GetConnected(); !ok {
			err := device.Connect()
			if err != nil {
				if strings.Contains(err.Error(), "Input/output error") {
					bluetoothResetChan <- true
					log.Error("unable to connect, bluetooth needs reset: ", err)
					stopRope.Cut()
					return
				}
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
			if attempts >= 20 {
				log.Errorf("unable to check whether services were resolved for '%s' after %d attempts", addr, attempts)
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
		deviceStopRope := NewRope()
		bleLight := NewBleLight(rgbChar, notifyChar, statusChan, deviceStopRope)

		mqttClient.Subscribe(colorTopic, 2, GetMessageHandlerSetColor(&bleLight))
		mqttClient.Subscribe(modeTopic, 2, GetMessageHandlerSetMode(&bleLight))
		mqttClient.Subscribe(powerTopic, 2, GetMessageHandlerSetPower(&bleLight))

		go requestDeviceUpdates(&bleLight, deviceStopRope, bluetoothResetChan)
		go StatusChanPublisher(mountpoint, &mqttClient, statusChan, deviceStopRope)

		mqttClient.Publish(onlineTopic, 1, true, "true")
		log.Infof("successfully connected to '%s'", addr)

		err = bleLight.ListenNotifications()
		if err != nil {
			log.Errorf("error while listening for notifications from '%s': %v", addr, err)
		}

		select {
		case <-stopRope.WaitCut():
			// Global stop signal, disconnect
			deviceStopRope.Cut()
			deviceStopRope.WaitReleased()
			disconnectDevice(device)
			break OuterLoop
		case <-deviceStopRope.WaitCut():
			// Device disconnected, do not attempt reconnection.
			// It's best to just exit and restart the program externally, in order not to trigger this bug:
			// https://github.com/muka/go-bluetooth/issues/91
			log.Warningf("connection to '%s' lost, stopping program, please restart it.", addr)
			log.Warning("see https://github.com/muka/go-bluetooth/issues/91")
			deviceStopRope.WaitReleased()
			stopRope.Cut()
			disconnectDevice(device)
			break OuterLoop
		}
	}
	mqttClient.Unsubscribe(colorTopic, modeTopic, powerTopic)

}

func disconnectDevice(device *device2.Device1) {
	addr, _ := device.GetAddress()
	log.Debugf("disconnecting '%s'", addr)
	err := device.Disconnect()
	if err != nil {
		log.Errorf("unable to disconnect device on stop '%s': %v", addr, err)
	}
	device.Close()
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

	stopRope := NewRope()

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
	name, _ := adapter.GetAdapterID()
	log.Debugf("Bluetooth adapter: %s", name)

	if powered, _ := adapter.GetPowered(); !powered {
		log.Info("turning Bluetooth adapter on...")
		if err := adapter.SetPowered(true); err != nil {
			log.Fatal("unable to turn on adapter: ", err)
		}
	}

	log.Debug("waiting for one device to be discovered")
	if err := adapter.StartDiscovery(); err != nil {
		log.Warning("failed to start discovery")
	}
	scanChan, cancel, err := adapter.OnDeviceDiscovered()
	if err != nil {
		log.Fatal("failed to retrieve discovered devices channel: ", err)
	}
DiscoveryLoop:
	for {
		select {
		case discoveredDev := <-scanChan:
			device, err := device2.NewDevice1(discoveredDev.Path)
			if err != nil {
				log.Errorf("failed to retrieve discovered device '%s': %v", discoveredDev.Path, err)
				continue DiscoveryLoop
			}
			addr, _ := device.GetAddress()
			if _, ok := config.Devices[addr]; ok {
				log.Debugf("found device '%s', proceeding", addr)
				break DiscoveryLoop
			}
		case <-time.After(3 * time.Second):
			log.Warning("timeout, proceeding anyway")
			break DiscoveryLoop
		}
	}
	cancel()
	_ = adapter.StopDiscovery()

	bluetoothResetChan := make(chan bool)

	for addr, deviceConfig := range config.Devices {
		devMountpoint := path.Join(mountpoint, deviceConfig.MountPoint)
		go handleDeviceForever(adapter, addr, deviceConfig, devMountpoint, mqttClient, stopRope, bluetoothResetChan)
	}

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)
	go signalHandler(signalChan, stopRope)

	<-stopRope.WaitCut()

	select {
	case <-stopRope.WaitReleased():
	case <-time.After(5 * time.Second):
		log.Warning("timed out waiting for all goroutines to stop, potential deadlock")
	}

	select {
	case <-bluetoothResetChan:
		if config.Bluetooth != nil && config.Bluetooth.ResetProgram != nil {
			log.Warning("bluetooth reset was requested, resetting")
			if err := exec.Command(*config.Bluetooth.ResetProgram); err != nil {
				log.Error("unable to reset bluetooth: ", err)
			} else {
				time.Sleep(5 * time.Second)
				log.Info("bluetooth reset, exiting")
			}
		} else {
			log.Warning("bluetooth reset was requested, but it was not configured; please reset manually")
		}
	default:
		break
	}
}
