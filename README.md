# pia-wg-config

A Wireguard config generator for Private Internet Access.

## Usage

`go get -u github.com/kylegrantlucas/pia-wg-config`

`pia-wg-config -o wg0.conf USERNAME PASSWORD`

You can now use `wg0.conf` to connect using your favorite wireguard client.

## Background

Based off of the [manual-connections](https://github.com/pia-foss/manual-connections) scripts provided FOSS by Private Internet Access. 

Golang was chosen to provide stability and portability to the scripts.

`pia-wg-config` is entirely self-contained and does require any external files.