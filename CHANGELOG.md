# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [v1.2.0]

### Added
- `--ca-cert` / `-c` flag to specify a custom CA certificate file path for verifying PIA's WireGuard API endpoint
- Support for `PIAWGCONFIG_USER` and `PIAWGCONFIG_PW` environment variables as an alternative to positional username/password arguments
- Fail-fast CA certificate validation at client initialisation (bad `--ca-cert` is caught before any network calls)
- Username format validation — PIA usernames must match `^p\d+$`; a clear error is shown for non-matching input
- Unit tests for `GetToken()` (success, 401 unauthorised, empty token response, malformed JSON)
- Unit tests for `downloadPIACertificate()` (local file, missing local file, empty fingerprint blocks download)
- Nine CLI integration tests covering missing args, env-var credentials, bad `--ca-cert`, bad region, invalid username format, region listing, and help output
- Verbose logging: region source (user-specified vs default) is now reported with `-v`
- Verbose logging: WireGuard server details (IP, VIP, peer IP, DNS, port) reported after successful `AddKey` with `-v`
- Verbose logging: detailed certificate lifecycle events in `downloadPIACertificate` (cache hit, local file size, download progress, fingerprint value, cert subject and expiry)

### Changed
- PIA token endpoint migrated from regional metadata servers (`/authv3/generateToken`) to the official central API (`https://www.privateinternetaccess.com/api/client/v2/token`), matching the approach used by PIA's own desktop client and `manual-connections` scripts
- Removed dead metadata-server code (`metadataServers`, `getMetadataServerForRegion()`, `generateMetadataServerList()`) that was never reachable on current PIA infrastructure
- `executePIARequest()` no longer accepts or handles a token argument (token is passed via the URL only, as required by the WireGuard API)
- Updated README to document `--ca-cert` flag and environment-variable credential method

### Fixed
- Token generation failures caused by PIA's new server naming convention (`server-XXXXX-0a`) making regional metadata servers unreachable (fixes #12)

## [v1.1.1]

### Changed
- Updated Go version to 1.23.0 with toolchain 1.24.3
- Updated all dependencies to latest versions for security and compatibility

## [v1.1.0]

### Added
- `regions` command to list all available PIA regions
- Enhanced CLI help text to emphasize region configurability
- `GetAvailableRegions()` method to PIA client
- Comprehensive documentation improvements
- Contributing guidelines (CONTRIBUTING.md)
- Examples for popular regions in README
- Integration examples (Docker, Bash scripts)
- Troubleshooting section in documentation

### Changed
- Improved README with clear emphasis on region selection
- Enhanced CLI flag description for region parameter
- Better error messages and help text

### Fixed
- Clarified that regions are NOT hardcoded but configurable via CLI flags

## [Previous Versions]

### Added
- Initial release with Wireguard config generation
- Support for all PIA regions via `-r/--region` flag
- File output option via `-o/--outfile` flag
- Verbose logging option via `-v/--verbose` flag
- MIT License
- Basic README documentation

### Technical Details
- Built with Go 1.23+
- Uses PIA's official API endpoints
- Self-contained binary with no external dependencies
- Cross-platform compatibility