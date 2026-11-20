# pia-wg-config

Go CLI that generates WireGuard configs for Private Internet Access (PIA). Public repo (github.com/kylegrantlucas/pia-wg-config).

## Layout
- `main.go` — urfave/cli entrypoint (`regions` subcommand, `-r/--region`, `-o/--outfile`, `-v/--verbose`).
- `pia/pia.go` — PIA API client: server list (`serverlist.piaservers.net/vpninfo/servers/v6`), token (central `www.privateinternetaccess.com/api/client/v2/token`), `addKey` against the region's WG server (embedded PIA CA `pia/ca.rsa.4096.crt`, custom dialer); `pia/pia_test.go` unit tests.
- `pia/wg.go` — key generation + config rendering; `pia/wg_test.go` unit tests.

## Commands (run inside `nix develop` / direnv)
- Test: `go test ./...` · Vet: `go vet ./...` · Build: `go build` (or `make build` to stamp version)
- Lint: `golangci-lint run` (currently reports 6 pre-existing findings; not gated in CI)
- Smoke (network, no creds): `go run . regions`

## Invariants / gotchas
- Go 1.27 (go directive `1.27.0`, toolchain `go1.27.2`); flake pins nixpkgs `go_1_27` with `GOTOOLCHAIN=local`. Keep `.github/workflows/ci.yml` on the same version.
- No `vendor/` — modules are fetched normally.
- Never log credentials, tokens or private keys; generated `*.conf` files contain private keys and are gitignored. The addKey token travels in the URL query, so never surface a `*url.Error` from that request unmodified.
- Endpoints and response handling mirror PIA's `pia-foss/manual-connections` scripts (`get_region.sh`, `get_token.sh`, `connect_to_wireguard_with_token.sh`); check there first when PIA changes something. Config `Endpoint` is `server_ip:server_port` from the addKey response.
- Branch `wip/stdlib-cli` holds an unfinished stdlib-only CLI rewrite (drops urfave/cli, pkg/errors, dns).
