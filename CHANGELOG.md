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