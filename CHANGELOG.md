# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- `regions` command to list all available PIA regions
- Enhanced CLI help text to emphasize region configurability
- `GetAvailableRegions()` method to PIA client
- Comprehensive documentation improvements
- Contributing guidelines (CONTRIBUTING.md)
- Examples for popular regions in README
- Integration examples (Docker, Bash scripts)
- Troubleshooting section in documentation
- Generated configs name the Wireguard server in a `# ServerName: …` comment at the top, for gluetun's `SERVER_NAMES`
- Generated configs name the region in a `# Region: …` comment at the top
- `regions` marks regions that support port forwarding
- `-p/--port-forwarding` flag that fails unless the region supports port forwarding
- README section on using the config with gluetun and its PIA port forwarding

### Changed
- Improved README with clear emphasis on region selection
- Enhanced CLI flag description for region parameter
- Better error messages and help text

### Fixed
- Clarified that regions are NOT hardcoded but configurable via CLI flags
- Use PIA's v6 server list (as in manual-connections) and skip offline regions
- `regions` no longer depends on the default region existing
- Embed PIA's CA certificate instead of downloading it from GitHub at runtime
- Reject addKey responses whose status is not `OK` or that lack required fields
- Config `Endpoint` uses the `server_port` returned by addKey instead of a hard-coded `1337`
- Verbose output no longer prints the Wireguard private key, and errors no longer include the auth token
- README and help examples use region IDs that exist (e.g. `uk`, `de-frankfurt`)

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