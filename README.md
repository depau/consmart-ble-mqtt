# Consmart/Triones BLE smart lights to MQTT bridge

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

## Configuration

```yaml
---
#bluetooth:
#  adapter: "hci2"  # only needed if you don't want the default adapter

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