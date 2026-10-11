# pia-wg-config

Go CLI that generates WireGuard configs for Private Internet Access (PIA). Public repo (github.com/kylegrantlucas/pia-wg-config).

## Layout
- `main.go` — urfave/cli entrypoint (`regions` subcommand, `-r/--region`, `-o/--outfile`, `-v/--verbose`).
- `pia/pia.go` — PIA API client: server list (`serverlist.piaservers.net/vpninfo/servers/v4`), token (central `www.privateinternetaccess.com/api/client/v2/token`), `addKey` against the region's WG server (pinned PIA CA, custom dialer).
- `pia/wg.go` — key generation + config rendering; `pia/wg_test.go` unit tests.

## Commands (run inside `nix develop` / direnv)
- Test: `go test ./...` · Vet: `go vet ./...` · Build: `go build` (or `make build` to stamp version)
- Lint: `golangci-lint run` (currently reports 6 pre-existing findings; not gated in CI)
- Smoke (network, no creds): `go run . regions`

## Invariants / gotchas
- Go 1.27 (go directive `1.27.0`, toolchain `go1.27.2`); flake pins nixpkgs `go_1_27` with `GOTOOLCHAIN=local`. Keep `.github/workflows/ci.yml` on the same version.
- No `vendor/` — modules are fetched normally.
- Never log credentials or tokens; generated `*.conf` files contain private keys and are gitignored.
- Branch `wip/stdlib-cli` holds an unfinished stdlib-only CLI rewrite (drops urfave/cli, pkg/errors, dns).
