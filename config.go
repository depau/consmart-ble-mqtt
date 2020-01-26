package main

import (
	"gopkg.in/yaml.v2"
	"io/ioutil"
)

type Config struct {
	Bluetooth *BluetoothConfig        `yaml:"bluetooth,omitempty"`
	MQTT      MQTTConfig              `yaml:"mqtt"`
	Devices   map[string]DeviceConfig `yaml:"devices"`
}

type TLSConfig struct {
	InsecureSkipVerify bool `yaml:"insecure,omitempty"`
}

type DeviceConfig struct {
	MountPoint           string   `yaml:"mountpoint"`
	RGBCharacteristic    *string  `yaml:"rgb_characteristic,omitempty"`
	NotifyCharacteristic *string  `yaml:"notify_characteristic,omitempty"`
	ReadStatusInterval   *float64 `yaml:"read_status_interval,omitempty"`
}

type BluetoothConfig struct {
	Adapter      *string `yaml:"adapter,omitempty"`
	ResetProgram *string `yaml:"reset_prog,omitempty"`
}

type MQTTConfig struct {
	MountPoint *string    `yaml:"mountpoint,omitempty"`
	Servers    []string   `yaml:"servers"`
	ClientID   *string    `yaml:"client_id,omitempty"`
	Username   *string    `yaml:"username,omitempty"`
	Password   *string    `yaml:"password,omitempty"`
	TLS        *TLSConfig `yaml:"tls,omitempty"`
}

func UnmarshalConfig(yml []byte, config *Config) (err error) {
	err = yaml.Unmarshal(yml, config)
	return
}

func ReadConfig(path string) (config Config, err error) {
	var content []byte
	content, err = ioutil.ReadFile(path)
	if err != nil {
		return
	}
	err = UnmarshalConfig(content, &config)
	return
}
