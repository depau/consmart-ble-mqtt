# Consmart/Triones BLE smart lights to MQTT bridge

**Warning: crazy spaghetty code ahead**

Exposes Triones Bluetooth smart lights over MQTT.

It seems to work, reconnection does not however, due to a bug in go-bluetooth.

As a workaround it currently exits when one device disconnects, you can restart it.
Use persistent messages when publishing control messages so changes are applied when
it's back up.

## Notes on Bluez

Bluez requires a small patch for some lights to work. In fact they do not comply with
the GATT protocol and send an error with a `0` error code, which bluez does not know
how to handle.

[The patch](0001-Workaround-for-non-compliant-BLE-lights.patch) simply makes it ignore
the error, it should work just fine. The bluez team is still discussing how to handle
such misbehaving devices.

## Notes on Raspberry Pi

Before we start, I want you to be aware of this: Raspberry Pi is the worst thing humanity
ever pulled off, the worst SBC on Earth, you should sell it or return it to Amazon ASAP
and get a RockPro64 instead.

Let that sink in.

Really. If Broadcom makes something, you already know how stable it's going to be. But
the guys at Raspberry Pi didn't think it was unstable enough, so they decided it was a
good idea to use an SD card as the one and only storage device of the computer. That
thing is a horrible nightmare and you should burn it.

That being said, I implemented a workaround in order to reset the Bluetooth chip when
it goes woo-woo. It works on Arch Linux ARM with the mainline `aarch64` kernel, it
might need changes for other kernels.

```
make raspi-mainline-reset-bt
make suid
```

Then add `reset_prog` as shown below to the configuration. The reset program will be
called if needed right before the program exits after failure.

## Configuration

```yaml
---
#bluetooth:
#  adapter: "hci2"                  # only needed if you don't want the default adapter
#  reset_prog: "path/to/executable" # helper to reset Bluetooth in case it goes crazy

mqtt:
  mountpoint: "kitchen/lights"
  servers:
    - 'tcp://localhost:1883'
    #- 'tcp://otherserver:1883'
    #- 'ws://websocket:80'
  client_id: 'blelight2mqtt'
  #username: "user"
  #password: "password"
  #tls:  # do not add section if you don't want TLS
  #  insecure_skip_verify: false

devices:
  'DE:AD:BE:EF:D0:0D':
    mountpoint: 'friendly_name/'
```

## Usage

### Building

```bash
go build
```

### Running

```bash
./consmart-ble-mqtt config.yml
```
