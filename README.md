This is a fork https://github.com/kylegrantlucas/pia-wg-config original script.

I have updated it to use the latest PIA API and added the flag `--server, -s` which will add the server's common name to the config file. This is useful for adding it to Gluetun for port-forwarding.

# pia-wg-config

A Wireguard config generator for Private Internet Access.

## Usage

`go install github.com/Ephemeral-Dust/pia-wg-config@latest`

`pia-wg-config -o wg0.conf USERNAME PASSWORD`

You can now use `wg0.conf` to connect using your favorite wireguard client.

## Background

Based off of the [manual-connections](https://github.com/pia-foss/manual-connections) scripts provided FOSS by Private Internet Access.

Golang was chosen to provide stability and portability to the scripts.

`pia-wg-config` is entirely self-contained and does require any external files.
