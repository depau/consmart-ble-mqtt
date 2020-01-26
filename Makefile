#!/usr/bin/make

all: raspi-mainline-reset-bt consmart-ble-mqtt

%: %.c
	$(CC) -o $@ $<

consmart-ble-mqtt:
	go build

suid: raspi-mainline-reset-bt
	sudo chown root:root raspi-mainline-reset-bt
	sudo chmod +S raspi-mainline-reset-bt

.PHONY: consmart-ble-mqtt
.DEFAULT: all
