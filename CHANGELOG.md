# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Modern Go patterns with context support and structured logging
- Custom error types for better error handling (`RegionValidationError`, `AuthenticationError`, `NetworkError`)
- Timeout configuration for API requests
- Enhanced Wireguard key generation with proper error handling
- Comprehensive input validation and sanitization
- Structured configuration building with `WgConfig` type
- Lightweight custom CLI implementation replacing external framework

### Changed
- Modernized codebase to use Go 1.23+ features
- Replaced `log` package with structured logging (`slog`)
- Added context-aware operations throughout the application
- Improved CLI with better error messages and user guidance
- Enhanced test suite with modern Go testing patterns
- Refactored PIA client with cleaner API design
- Updated Wireguard generator with type-safe configuration building
- **Massive dependency reduction**: Removed CLI framework, DNS library, and Wireguard library
- Replaced external dependencies with standard library implementations
- Reduced binary size by ~30% (11.8MB → 8.2MB)
- Custom hostname-to-IP mapping replaces DNS library
- Standard library `curve25519` replaces Wireguard library for key generation
- Custom flag parsing replaces CLI framework

### Fixed
- Improved error handling with proper error wrapping
- Better network timeout handling
- More secure file permissions (0600) for generated configs
- Enhanced DNS server validation and fallback handling

### Dependencies
- **Before**: 4 external dependencies (`benburkert/dns`, `urfave/cli/v2`, `wireguard/wgctrl`, `pkg/errors`)
- **After**: 1 external dependency (`golang.org/x/crypto` for curve25519 only)
- All other functionality implemented with Go standard library

## [1.1.1] - 2025-01-20

### Changed
- Updated all dependencies to latest versions
- Updated Go version requirement to 1.23+
- Refreshed vendored dependencies for security and compatibility

## [1.1.0] - 2025-01-20

### Added
- `regions` command to list all available PIA regions
- Enhanced CLI help text to emphasize region configurability
- `GetAvailableRegions()` method to PIA client
- Comprehensive documentation improvements
- Examples for popular regions in README
- Integration examples (Docker, Bash scripts)
- Troubleshooting section in documentation
- GitHub issue templates and PR templates
- Development tooling (Makefile, CHANGELOG)

### Changed
- Improved README with clear emphasis on region selection
- Enhanced CLI flag description for region parameter
- Better error messages and help text
- Updated .gitignore for better development workflow

### Fixed
- Clarified that regions are NOT hardcoded but configurable via CLI flags
- Fixed region mapping bug in server list generation
- Added region validation with helpful error messages

## [Previous Versions]

### Added
- Initial release with Wireguard config generation
- Support for all PIA regions via `-r/--region` flag
- File output option via `-o/--outfile` flag
- Verbose logging option
- MIT License
- Basic README documentation

### Technical Details
- Built with Go 1.23+
- Uses PIA's official API endpoints
- Self-contained binary with no external dependencies
- Cross-platform compatibility