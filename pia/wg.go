package pia

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log/slog"
	"strings"

	"golang.org/x/crypto/curve25519"
)

// WgGenerator handles Wireguard configuration generation
type WgGenerator struct {
	client *PIAClient
	logger *slog.Logger
}

// WgConfig represents a Wireguard configuration
type WgConfig struct {
	PrivateKey string
	PublicKey  string
	Address    string
	DNS        []string
	Peer       WgPeer
}

// WgPeer represents a Wireguard peer configuration
type WgPeer struct {
	PublicKey  string
	Endpoint   string
	AllowedIPs []string
}

// NewWgGenerator creates a new Wireguard configuration generator
func NewWgGenerator(client *PIAClient, logger *slog.Logger) *WgGenerator {
	if logger == nil {
		logger = slog.Default()
	}

	return &WgGenerator{
		client: client,
		logger: logger.With("component", "wg-generator"),
	}
}

// Generate creates a complete Wireguard configuration
func (w *WgGenerator) Generate(ctx context.Context, token string) (string, error) {
	w.logger.Debug("Starting Wireguard configuration generation")

	// Generate key pair
	privateKey, publicKey, err := w.generateKeyPair()
	if err != nil {
		return "", fmt.Errorf("failed to generate key pair: %w", err)
	}

	w.logger.Debug("Generated Wireguard key pair")

	// Add key to PIA and get server details
	result, err := w.client.AddKey(ctx, token, publicKey)
	if err != nil {
		return "", fmt.Errorf("failed to add key to PIA: %w", err)
	}

	// Validate the response
	if err := w.validateAddKeyResult(result); err != nil {
		return "", fmt.Errorf("invalid response from PIA: %w", err)
	}

	// Build configuration
	config := w.buildConfig(privateKey, result)
	configText := w.formatConfig(config)

	w.logger.Info("Successfully generated Wireguard configuration",
		"server_ip", result.ServerIP,
		"peer_ip", result.PeerIP)

	return configText, nil
}

// generateKeyPair creates a new Wireguard private/public key pair
func (w *WgGenerator) generateKeyPair() (privateKey, publicKey string, err error) {
	// Generate 32 random bytes for private key
	privateKeyBytes := make([]byte, 32)
	if _, err := rand.Read(privateKeyBytes); err != nil {
		return "", "", fmt.Errorf("failed to generate private key: %w", err)
	}

	// Clamp the private key for curve25519
	privateKeyBytes[0] &= 248
	privateKeyBytes[31] &= 127
	privateKeyBytes[31] |= 64

	// Generate public key from private key
	publicKeyBytes, err := curve25519.X25519(privateKeyBytes, curve25519.Basepoint)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate public key: %w", err)
	}

	// Encode to base64
	privateKey = base64.StdEncoding.EncodeToString(privateKeyBytes)
	publicKey = base64.StdEncoding.EncodeToString(publicKeyBytes)

	return privateKey, publicKey, nil
}

// validateAddKeyResult checks if the PIA response is valid
func (w *WgGenerator) validateAddKeyResult(result AddKeyResult) error {
	if result.Status != "OK" {
		return fmt.Errorf("PIA server returned status: %s", result.Status)
	}

	if result.ServerKey == "" {
		return fmt.Errorf("empty server key in response")
	}

	if result.ServerIP == "" {
		return fmt.Errorf("empty server IP in response")
	}

	if result.PeerIP == "" {
		return fmt.Errorf("empty peer IP in response")
	}

	if result.ServerPort == 0 {
		return fmt.Errorf("invalid server port in response")
	}

	return nil
}

// buildConfig creates a WgConfig from the PIA response
func (w *WgGenerator) buildConfig(privateKey string, result AddKeyResult) WgConfig {
	return WgConfig{
		PrivateKey: privateKey,
		PublicKey:  result.PeerPubkey,
		Address:    result.PeerIP,
		DNS:        w.formatDNSServers(result.DNSServers),
		Peer: WgPeer{
			PublicKey:  result.ServerKey,
			Endpoint:   fmt.Sprintf("%s:%d", result.ServerIP, result.ServerPort),
			AllowedIPs: []string{"0.0.0.0/0"},
		},
	}
}

// formatDNSServers processes DNS servers from PIA response
func (w *WgGenerator) formatDNSServers(dnsServers []string) []string {
	if len(dnsServers) == 0 {
		// Default to reliable public DNS if none provided
		return []string{"1.1.1.1", "1.0.0.1"}
	}

	// Filter out empty entries and validate
	var validDNS []string
	for _, dns := range dnsServers {
		dns = strings.TrimSpace(dns)
		if dns != "" && w.isValidIP(dns) {
			validDNS = append(validDNS, dns)
		}
	}

	if len(validDNS) == 0 {
		// Fallback to public DNS if all entries were invalid
		return []string{"1.1.1.1", "1.0.0.1"}
	}

	return validDNS
}

// isValidIP performs basic IP address validation
func (w *WgGenerator) isValidIP(ip string) bool {
	parts := strings.Split(ip, ".")
	if len(parts) != 4 {
		return false
	}

	for _, part := range parts {
		if len(part) == 0 || len(part) > 3 {
			return false
		}
		// Basic numeric check - could be more thorough
		for _, char := range part {
			if char < '0' || char > '9' {
				return false
			}
		}
	}
	return true
}

// formatConfig converts WgConfig to Wireguard configuration text
func (w *WgGenerator) formatConfig(config WgConfig) string {
	var builder strings.Builder

	// Interface section
	builder.WriteString("[Interface]\n")
	builder.WriteString(fmt.Sprintf("PrivateKey = %s\n", config.PrivateKey))
	builder.WriteString(fmt.Sprintf("Address = %s\n", config.Address))

	if len(config.DNS) > 0 {
		builder.WriteString(fmt.Sprintf("DNS = %s\n", strings.Join(config.DNS, ", ")))
	}

	builder.WriteString("\n")

	// Peer section
	builder.WriteString("[Peer]\n")
	builder.WriteString(fmt.Sprintf("PublicKey = %s\n", config.Peer.PublicKey))
	builder.WriteString(fmt.Sprintf("Endpoint = %s\n", config.Peer.Endpoint))
	builder.WriteString(fmt.Sprintf("AllowedIPs = %s\n", strings.Join(config.Peer.AllowedIPs, ", ")))
	builder.WriteString("PersistentKeepalive = 25\n")

	return builder.String()
}

// GenerateRandomBytes generates cryptographically secure random bytes
func GenerateRandomBytes(length int) ([]byte, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return nil, fmt.Errorf("failed to generate random bytes: %w", err)
	}
	return bytes, nil
}
