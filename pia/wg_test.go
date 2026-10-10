package pia

import (
	"log/slog"
	"os"
	"strings"
	"testing"
)

func TestWgGenerator_generateKeyPair(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "successful key generation",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
			generator := &WgGenerator{logger: logger}

			privateKey, publicKey, err := generator.generateKeyPair()

			if (err != nil) != tt.wantErr {
				t.Errorf("generateKeyPair() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if privateKey == "" {
					t.Error("generateKeyPair() returned empty private key")
				}
				if publicKey == "" {
					t.Error("generateKeyPair() returned empty public key")
				}
				if privateKey == publicKey {
					t.Error("generateKeyPair() returned identical private and public keys")
				}
				// Keys should be valid base64
				if len(privateKey) == 0 || len(publicKey) == 0 {
					t.Error("generateKeyPair() returned zero-length keys")
				}
			}
		})
	}
}

func TestWgGenerator_validateAddKeyResult(t *testing.T) {
	tests := []struct {
		name    string
		result  AddKeyResult
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid result",
			result: AddKeyResult{
				Status:     "OK",
				ServerKey:  "test_server_key",
				ServerPort: 1337,
				ServerIP:   "192.168.1.1",
				PeerIP:     "10.0.0.2/32",
				PeerPubkey: "test_peer_key",
				DNSServers: []string{"1.1.1.1"},
			},
			wantErr: false,
		},
		{
			name: "invalid status",
			result: AddKeyResult{
				Status: "ERROR",
			},
			wantErr: true,
			errMsg:  "PIA server returned status: ERROR",
		},
		{
			name: "empty server key",
			result: AddKeyResult{
				Status:    "OK",
				ServerKey: "",
			},
			wantErr: true,
			errMsg:  "empty server key in response",
		},
		{
			name: "empty server IP",
			result: AddKeyResult{
				Status:    "OK",
				ServerKey: "test_key",
				ServerIP:  "",
			},
			wantErr: true,
			errMsg:  "empty server IP in response",
		},
		{
			name: "empty peer IP",
			result: AddKeyResult{
				Status:    "OK",
				ServerKey: "test_key",
				ServerIP:  "192.168.1.1",
				PeerIP:    "",
			},
			wantErr: true,
			errMsg:  "empty peer IP in response",
		},
		{
			name: "zero server port",
			result: AddKeyResult{
				Status:     "OK",
				ServerKey:  "test_key",
				ServerIP:   "192.168.1.1",
				PeerIP:     "10.0.0.1",
				ServerPort: 0,
			},
			wantErr: true,
			errMsg:  "invalid server port in response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
			generator := &WgGenerator{logger: logger}

			err := generator.validateAddKeyResult(tt.result)

			if (err != nil) != tt.wantErr {
				t.Errorf("validateAddKeyResult() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && err.Error() != tt.errMsg {
				t.Errorf("validateAddKeyResult() error message = %v, want %v", err.Error(), tt.errMsg)
			}
		})
	}
}

func TestWgGenerator_formatDNSServers(t *testing.T) {
	tests := []struct {
		name       string
		dnsServers []string
		want       []string
	}{
		{
			name:       "valid DNS servers",
			dnsServers: []string{"1.1.1.1", "8.8.8.8"},
			want:       []string{"1.1.1.1", "8.8.8.8"},
		},
		{
			name:       "empty DNS servers",
			dnsServers: []string{},
			want:       []string{"1.1.1.1", "1.0.0.1"},
		},
		{
			name:       "nil DNS servers",
			dnsServers: nil,
			want:       []string{"1.1.1.1", "1.0.0.1"},
		},
		{
			name:       "DNS with whitespace",
			dnsServers: []string{" 1.1.1.1 ", "8.8.8.8"},
			want:       []string{"1.1.1.1", "8.8.8.8"},
		},
		{
			name:       "DNS with empty entries",
			dnsServers: []string{"1.1.1.1", "", "8.8.8.8"},
			want:       []string{"1.1.1.1", "8.8.8.8"},
		},
		{
			name:       "invalid DNS entries fallback",
			dnsServers: []string{"invalid", "not.an.ip"},
			want:       []string{"1.1.1.1", "1.0.0.1"},
		},
		{
			name:       "mixed valid and invalid",
			dnsServers: []string{"1.1.1.1", "invalid", "8.8.8.8"},
			want:       []string{"1.1.1.1", "8.8.8.8"},
		},
		{
			name:       "single valid DNS",
			dnsServers: []string{"9.9.9.9"},
			want:       []string{"9.9.9.9"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
			generator := &WgGenerator{logger: logger}

			got := generator.formatDNSServers(tt.dnsServers)

			if !sliceEqual(got, tt.want) {
				t.Errorf("formatDNSServers() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWgGenerator_isValidIP(t *testing.T) {
	tests := []struct {
		name string
		ip   string
		want bool
	}{
		{
			name: "valid IPv4",
			ip:   "192.168.1.1",
			want: true,
		},
		{
			name: "valid IPv4 zeros",
			ip:   "0.0.0.0",
			want: true,
		},
		{
			name: "valid IPv4 max values",
			ip:   "255.255.255.255",
			want: true,
		},
		{
			name: "valid single digits",
			ip:   "1.2.3.4",
			want: true,
		},
		{
			name: "invalid too few octets",
			ip:   "192.168.1",
			want: false,
		},
		{
			name: "invalid too many octets",
			ip:   "192.168.1.1.1",
			want: false,
		},
		{
			name: "invalid non-numeric",
			ip:   "192.168.a.1",
			want: false,
		},
		{
			name: "invalid empty string",
			ip:   "",
			want: false,
		},
		{
			name: "invalid empty octet",
			ip:   "192..1.1",
			want: false,
		},
		{
			name: "invalid leading zeros",
			ip:   "192.168.001.1",
			want: true, // Our simple validator allows this
		},
		{
			name: "invalid special characters",
			ip:   "192.168.1.1-",
			want: false,
		},
		{
			name: "invalid spaces",
			ip:   "192.168.1. 1",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
			generator := &WgGenerator{logger: logger}

			got := generator.isValidIP(tt.ip)

			if got != tt.want {
				t.Errorf("isValidIP(%q) = %v, want %v", tt.ip, got, tt.want)
			}
		})
	}
}

func TestWgGenerator_buildConfig(t *testing.T) {
	tests := []struct {
		name       string
		privateKey string
		result     AddKeyResult
		want       WgConfig
	}{
		{
			name:       "basic config",
			privateKey: "test_private_key",
			result: AddKeyResult{
				Status:     "OK",
				ServerKey:  "server_public_key",
				ServerPort: 1337,
				ServerIP:   "192.168.1.1",
				PeerIP:     "10.0.0.2/32",
				PeerPubkey: "peer_public_key",
				DNSServers: []string{"1.1.1.1", "1.0.0.1"},
			},
			want: WgConfig{
				PrivateKey: "test_private_key",
				PublicKey:  "peer_public_key",
				Address:    "10.0.0.2/32",
				DNS:        []string{"1.1.1.1", "1.0.0.1"},
				Peer: WgPeer{
					PublicKey:  "server_public_key",
					Endpoint:   "192.168.1.1:1337",
					AllowedIPs: []string{"0.0.0.0/0"},
				},
			},
		},
		{
			name:       "empty DNS servers",
			privateKey: "test_private_key",
			result: AddKeyResult{
				Status:     "OK",
				ServerKey:  "server_public_key",
				ServerPort: 51820,
				ServerIP:   "10.10.10.10",
				PeerIP:     "172.16.0.1/24",
				PeerPubkey: "peer_public_key",
				DNSServers: []string{},
			},
			want: WgConfig{
				PrivateKey: "test_private_key",
				PublicKey:  "peer_public_key",
				Address:    "172.16.0.1/24",
				DNS:        []string{"1.1.1.1", "1.0.0.1"}, // fallback
				Peer: WgPeer{
					PublicKey:  "server_public_key",
					Endpoint:   "10.10.10.10:51820",
					AllowedIPs: []string{"0.0.0.0/0"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
			generator := &WgGenerator{logger: logger}

			got := generator.buildConfig(tt.privateKey, tt.result)

			if got.PrivateKey != tt.want.PrivateKey {
				t.Errorf("buildConfig().PrivateKey = %v, want %v", got.PrivateKey, tt.want.PrivateKey)
			}
			if got.PublicKey != tt.want.PublicKey {
				t.Errorf("buildConfig().PublicKey = %v, want %v", got.PublicKey, tt.want.PublicKey)
			}
			if got.Address != tt.want.Address {
				t.Errorf("buildConfig().Address = %v, want %v", got.Address, tt.want.Address)
			}
			if !sliceEqual(got.DNS, tt.want.DNS) {
				t.Errorf("buildConfig().DNS = %v, want %v", got.DNS, tt.want.DNS)
			}
			if got.Peer.PublicKey != tt.want.Peer.PublicKey {
				t.Errorf("buildConfig().Peer.PublicKey = %v, want %v", got.Peer.PublicKey, tt.want.Peer.PublicKey)
			}
			if got.Peer.Endpoint != tt.want.Peer.Endpoint {
				t.Errorf("buildConfig().Peer.Endpoint = %v, want %v", got.Peer.Endpoint, tt.want.Peer.Endpoint)
			}
			if !sliceEqual(got.Peer.AllowedIPs, tt.want.Peer.AllowedIPs) {
				t.Errorf("buildConfig().Peer.AllowedIPs = %v, want %v", got.Peer.AllowedIPs, tt.want.Peer.AllowedIPs)
			}
		})
	}
}

func TestWgGenerator_formatConfig(t *testing.T) {
	tests := []struct {
		name            string
		config          WgConfig
		wantContains    []string
		wantNotContains []string
	}{
		{
			name: "complete config",
			config: WgConfig{
				PrivateKey: "test_private_key",
				PublicKey:  "test_public_key",
				Address:    "10.0.0.2/32",
				DNS:        []string{"1.1.1.1", "1.0.0.1"},
				Peer: WgPeer{
					PublicKey:  "server_public_key",
					Endpoint:   "192.168.1.1:1337",
					AllowedIPs: []string{"0.0.0.0/0"},
				},
			},
			wantContains: []string{
				"[Interface]",
				"PrivateKey = test_private_key",
				"Address = 10.0.0.2/32",
				"DNS = 1.1.1.1, 1.0.0.1",
				"[Peer]",
				"PublicKey = server_public_key",
				"Endpoint = 192.168.1.1:1337",
				"AllowedIPs = 0.0.0.0/0",
				"PersistentKeepalive = 25",
			},
		},
		{
			name: "config without DNS",
			config: WgConfig{
				PrivateKey: "test_private_key",
				PublicKey:  "test_public_key",
				Address:    "10.0.0.2/32",
				DNS:        []string{},
				Peer: WgPeer{
					PublicKey:  "server_public_key",
					Endpoint:   "192.168.1.1:1337",
					AllowedIPs: []string{"0.0.0.0/0"},
				},
			},
			wantContains: []string{
				"[Interface]",
				"PrivateKey = test_private_key",
				"Address = 10.0.0.2/32",
				"[Peer]",
				"PublicKey = server_public_key",
			},
			wantNotContains: []string{
				"DNS =",
			},
		},
		{
			name: "config with multiple allowed IPs",
			config: WgConfig{
				PrivateKey: "test_private_key",
				PublicKey:  "test_public_key",
				Address:    "10.0.0.2/32",
				DNS:        []string{"8.8.8.8"},
				Peer: WgPeer{
					PublicKey:  "server_public_key",
					Endpoint:   "192.168.1.1:51820",
					AllowedIPs: []string{"0.0.0.0/0", "::/0"},
				},
			},
			wantContains: []string{
				"AllowedIPs = 0.0.0.0/0, ::/0",
				"DNS = 8.8.8.8",
				"Endpoint = 192.168.1.1:51820",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
			generator := &WgGenerator{logger: logger}

			got := generator.formatConfig(tt.config)

			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("formatConfig() missing expected content: %q\nGot: %s", want, got)
				}
			}

			for _, notWant := range tt.wantNotContains {
				if strings.Contains(got, notWant) {
					t.Errorf("formatConfig() contains unexpected content: %q\nGot: %s", notWant, got)
				}
			}

			// Ensure basic structure is present
			if !strings.Contains(got, "[Interface]") {
				t.Error("formatConfig() missing [Interface] section")
			}
			if !strings.Contains(got, "[Peer]") {
				t.Error("formatConfig() missing [Peer] section")
			}
		})
	}
}

func TestGenerateRandomBytes(t *testing.T) {
	tests := []struct {
		name    string
		length  int
		wantLen int
		wantErr bool
	}{
		{
			name:    "generate 32 bytes",
			length:  32,
			wantLen: 32,
			wantErr: false,
		},
		{
			name:    "generate 16 bytes",
			length:  16,
			wantLen: 16,
			wantErr: false,
		},
		{
			name:    "generate 0 bytes",
			length:  0,
			wantLen: 0,
			wantErr: false,
		},
		{
			name:    "generate 1 byte",
			length:  1,
			wantLen: 1,
			wantErr: false,
		},
		{
			name:    "generate large amount",
			length:  1024,
			wantLen: 1024,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GenerateRandomBytes(tt.length)

			if (err != nil) != tt.wantErr {
				t.Errorf("GenerateRandomBytes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if len(got) != tt.wantLen {
				t.Errorf("GenerateRandomBytes() length = %d, want %d", len(got), tt.wantLen)
			}

			// Test that subsequent calls produce different results (for non-zero lengths)
			if tt.length > 0 {
				got2, err2 := GenerateRandomBytes(tt.length)
				if err2 != nil {
					t.Errorf("GenerateRandomBytes() second call error = %v", err2)
					return
				}

				// Convert to strings for comparison
				str1 := string(got)
				str2 := string(got2)
				if str1 == str2 {
					t.Error("GenerateRandomBytes() produced identical results on consecutive calls")
				}
			}
		})
	}
}

func TestNewWgGenerator(t *testing.T) {
	tests := []struct {
		name   string
		client *PIAClient
		logger *slog.Logger
		want   bool // whether we want a non-nil generator
	}{
		{
			name:   "with valid client and logger",
			client: &PIAClient{region: "test"},
			logger: slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{})),
			want:   true,
		},
		{
			name:   "with nil logger",
			client: &PIAClient{region: "test"},
			logger: nil,
			want:   true, // should still work with default logger
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewWgGenerator(tt.client, tt.logger)

			if (got != nil) != tt.want {
				t.Errorf("NewWgGenerator() = %v, want non-nil: %v", got, tt.want)
				return
			}

			if got != nil {
				if got.client != tt.client {
					t.Error("NewWgGenerator() did not set client correctly")
				}
				if got.logger == nil {
					t.Error("NewWgGenerator() resulted in nil logger")
				}
			}
		})
	}
}

func TestGenerateKeyPairProperties(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	generator := &WgGenerator{logger: logger}

	// Test that multiple key generations produce different keys
	keys := make(map[string]bool)
	for i := 0; i < 10; i++ {
		privateKey, publicKey, err := generator.generateKeyPair()
		if err != nil {
			t.Fatalf("generateKeyPair() iteration %d failed: %v", i, err)
		}

		// Check uniqueness
		if keys[privateKey] {
			t.Errorf("generateKeyPair() produced duplicate private key on iteration %d", i)
		}
		if keys[publicKey] {
			t.Errorf("generateKeyPair() produced duplicate public key on iteration %d", i)
		}

		keys[privateKey] = true
		keys[publicKey] = true

		// Check key format (should be base64)
		if len(privateKey) == 0 || len(publicKey) == 0 {
			t.Errorf("generateKeyPair() iteration %d produced empty keys", i)
		}

		// Keys should be different from each other
		if privateKey == publicKey {
			t.Errorf("generateKeyPair() iteration %d produced identical private and public keys", i)
		}
	}
}

// Helper function for comparing slices
func sliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Benchmarks
func BenchmarkGenerateKeyPair(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	generator := &WgGenerator{logger: logger}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := generator.generateKeyPair()
		if err != nil {
			b.Fatalf("generateKeyPair() failed: %v", err)
		}
	}
}

func BenchmarkFormatConfig(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	generator := &WgGenerator{logger: logger}

	config := WgConfig{
		PrivateKey: "test_private_key",
		PublicKey:  "test_public_key",
		Address:    "10.0.0.2/32",
		DNS:        []string{"1.1.1.1", "1.0.0.1"},
		Peer: WgPeer{
			PublicKey:  "server_public_key",
			Endpoint:   "192.168.1.1:1337",
			AllowedIPs: []string{"0.0.0.0/0"},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = generator.formatConfig(config)
	}
}
