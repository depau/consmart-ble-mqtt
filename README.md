# Consmart/Triones/Flyidea BLE smart lights to MQTT bridge

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

## MQTT topics

### Control

The light can be controlled by writing to topics under `{global_mountpoint}/{device_mountpoint}/control`.
There are 3 topics:

#### `control/power`

`on`/`off`

#### `control/color`

Takes an RGB color in the form `R,G,B`, for example `250,134,17` is a warm white.

Channels can go up to 255, `255,255,255` is white.

When saturation is zero (that is when all channels are set to the same value), the
light is set to use the dedicated white LEDs. White LEDs are brighter than the RGB ones.

When the color is set to `0,0,0`, the light is turned off.

#### `control/mode`

Takes a value in the form `light_mode,speed`.

Available modes are:

- `smooth rainbow`
- `pulsating red`
- `pulsating green`
- `pulsating blue`
- `pulsating yellow`
- `pulsating cyan`
- `pulsating magenta`
- `pulsating white`
- `pulsating red/green`
- `pulsating red/blue`
- `pulsating green/blue`
- `rainbow strobe`
- `red strobe`
- `green strobe`
- `blue strobe`
- `yellow strobe`
- `cyan strobe`
- `magenta strobe`
- `white strobe`
- `hard rainbow`

Bonus modes (not available in the app, but implemented by the lights):

- `pulsating RGB`
- `RGB strobe`
- `hard RGB`

Speed can go from 1 to 31, and is **inversely proportional**. That is when it's
set to 1 it's fast, when it's set to 31 you can barely see it changing.

Don't ask me about it, ask the guys who made these shitty lights.

Example: `smooth rainbow,3`


### Status

Status is reported to `{global_mountpoint}/{device_mountpoint}/status`.

Format is the same as the control channels but with a few exceptions:

Mode may also report `white` and `rgb`, which are not settable with the control
topic. In order to set it to white mode, set the color to a color with no
saturation (such as white). In order to set it to RGB mode, simply set a color.

When in `white` or `rgb` mode, the speed is not reported (it doesn't really make
any sense).

When a mode is enabled, the color changes are also reported as well roughly every
second.

## Unsupported features

There are some extra features that the lights support that have not been implemented:

- Clock/date setting (for auto on/off)
- Auto on/off
- "Passcode"
  - Yes, apparently if you reverse-engineer the app you can see that the SDK they
    used supports setting a 4-digit passcode. It is unclear how it actually works,
    it doesn't seem to be the Bluetooth pairing code, also it seems to me that it
    might be resettable without even providing it.
    It looks like if the passcode is set it tries to use one that's stored somewhere
    on the phone, but if it fails it tries to reset it. 🤷
    If this is not true it might actually be useful to make sure your neighbors
    don't have fun with your lights, but luckily my neighbors are not that techy.

## Troubleshooting

### `unable to retrieve RGB characteristic for '...'`

Have you rebuilt Bluez with my patch applied? See [Notes on Bluez](#notes-on-bluez)

### `unable to check whether services were resolved for '...'`

That's usually not an issue, it sometimes takes up to 30 seconds to connect to a
lightbulb.

If it doesn't connect after waiting a minute or so, try removing power to the
lights for a few seconds and see if they come back to life.

### `Input/output error`

Your Bluetooth adapter is stuck. If you're using a Raspberry Pi, see
[Notes on Raspberry Pi](#notes-on-raspberry-pi) for a workaround that doesn't
involve rebooting it.

Otherwise reboot or unplug-replug the adapter and restart the program.

### Program exits after lights disconnect

If lights decide it's time for a break, or you remove power from the lights,
the program will try to handle it by exiting.

That is because the `go-bluetooth` library that's used under the hood never
stops listening for notifications from the lights even after they disconnect,
causing a deadlock.

It will try to exit so you can have something else (i.e. a script or systemd)
restart it, and a new connection attempt will be performed.

### Lights stop responding but they're still connected

I haven't had many chances to debug this because it always happens after many
hours. That usually goes away after power-cycling the lights externally and
restarting the program.

I'm pretty sure this is the lights fault. I already regret buying these lights
so if you want some reliable lights, get the IKEA ones. They're 4 times as
expensive but they work like a charm.

I'm also working with some friends to build a modded dimmer and use it as a
cheap `zigbee2mqtt`-compatible gateway. We're still at early stages but
check that out: https://github.com/tradfree-mod

