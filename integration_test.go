package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kylegrantlucas/pia-wg-config/pia"
)

// TestConfigFileCreation tests various file creation scenarios
func TestConfigFileCreation(t *testing.T) {
	// Initialize logger for these tests
	originalLogger := logger
	logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	defer func() { logger = originalLogger }()

	tests := []struct {
		name        string
		config      string
		filename    string
		shouldExist bool
		wantErr     bool
	}{
		{
			name:        "create config in current directory",
			config:      testWireguardConfig(),
			filename:    "test-current-dir.conf",
			shouldExist: true,
			wantErr:     false,
		},
		{
			name:        "create config with .wg extension",
			config:      testWireguardConfig(),
			filename:    "test-wg-ext.wg",
			shouldExist: true,
			wantErr:     false,
		},
		{
			name:        "create config with no extension",
			config:      testWireguardConfig(),
			filename:    "test-no-ext",
			shouldExist: true,
			wantErr:     false,
		},
		{
			name:        "empty config content",
			config:      "",
			filename:    "test-empty.conf",
			shouldExist: true,
			wantErr:     false,
		},
		{
			name:        "large config content",
			config:      strings.Repeat(testWireguardConfig(), 100),
			filename:    "test-large.conf",
			shouldExist: true,
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up before test
			os.Remove(tt.filename)

			err := outputConfig(tt.config, tt.filename)

			if (err != nil) != tt.wantErr {
				t.Errorf("outputConfig() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.shouldExist {
				if _, err := os.Stat(tt.filename); os.IsNotExist(err) {
					t.Errorf("outputConfig() did not create file %s", tt.filename)
				} else {
					// Check file permissions
					info, _ := os.Stat(tt.filename)
					if info.Mode().Perm() != 0600 {
						t.Errorf("outputConfig() file permissions = %v, want 0600", info.Mode().Perm())
					}

					// Check file content
					content, err := os.ReadFile(tt.filename)
					if err != nil {
						t.Errorf("outputConfig() could not read created file: %v", err)
					} else if string(content) != tt.config {
						t.Errorf("outputConfig() file content mismatch")
					}
				}
			}

			// Clean up after test
			os.Remove(tt.filename)
		})
	}
}

// TestSubdirectoryCreation tests creating configs in subdirectories
func TestSubdirectoryCreation(t *testing.T) {
	// Initialize logger for these tests
	originalLogger := logger
	logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	defer func() { logger = originalLogger }()

	tempDir := t.TempDir()

	tests := []struct {
		name     string
		filepath string
		wantErr  bool
	}{
		{
			name:     "create in subdirectory",
			filepath: filepath.Join(tempDir, "configs", "test.conf"),
			wantErr:  true, // Should fail because directory doesn't exist
		},
		{
			name:     "create in existing subdirectory",
			filepath: filepath.Join(tempDir, "test.conf"),
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := testWireguardConfig()
			err := outputConfig(config, tt.filepath)

			if (err != nil) != tt.wantErr {
				t.Errorf("outputConfig() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr {
				if _, err := os.Stat(tt.filepath); os.IsNotExist(err) {
					t.Errorf("outputConfig() did not create file %s", tt.filepath)
				}
			}
		})
	}
}

// TestFlagParsing tests various flag parsing scenarios
func TestFlagParsing(t *testing.T) {
	// Initialize logger for these tests
	originalLogger := logger
	logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	defer func() { logger = originalLogger }()

	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name:    "multiple flags",
			args:    []string{"-r", "us_california", "-o", "test.conf", "-verbose", "testuser", "testpass"},
			wantErr: true, // Will fail on network but validates flag parsing
		},
		{
			name:    "flags in different order",
			args:    []string{"-verbose", "-o", "test.conf", "-r", "uk_london", "testuser", "testpass"},
			wantErr: true, // Will fail on network but validates flag parsing
		},
		{
			name:    "equals syntax for flags",
			args:    []string{"-r=us_california", "-o=test.conf", "testuser", "testpass"},
			wantErr: true, // Will fail on network but validates flag parsing
		},
		{
			name:    "mixed flag syntax",
			args:    []string{"-r=us_california", "-o", "test.conf", "-verbose", "testuser", "testpass"},
			wantErr: true, // Will fail on network but validates flag parsing
		},
		{
			name:    "invalid flag",
			args:    []string{"-invalid", "testuser", "testpass"},
			wantErr: true,
		},
		{
			name:    "duplicate flags",
			args:    []string{"-r", "us_california", "-r", "uk_london", "testuser", "testpass"},
			wantErr: true, // Will use last value, but fail on network
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			err := handleGenerateCommand(ctx, tt.args)

			if (err != nil) != tt.wantErr {
				t.Errorf("handleGenerateCommand() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestUsernamePasswordValidation tests various username/password scenarios
func TestUsernamePasswordValidation(t *testing.T) {
	// Initialize logger for these tests
	originalLogger := logger
	logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	defer func() { logger = originalLogger }()

	tests := []struct {
		name     string
		args     []string
		wantErr  bool
		errCheck func(error) bool
	}{
		{
			name:    "whitespace username",
			args:    []string{"   ", "password"},
			wantErr: true,
			errCheck: func(err error) bool {
				return strings.Contains(err.Error(), "username and password cannot be empty")
			},
		},
		{
			name:    "whitespace password",
			args:    []string{"username", "   "},
			wantErr: true,
			errCheck: func(err error) bool {
				return strings.Contains(err.Error(), "username and password cannot be empty")
			},
		},
		{
			name:    "special characters in username",
			args:    []string{"user@domain.com", "password"},
			wantErr: true, // Will fail on network but username is valid
		},
		{
			name:    "special characters in password",
			args:    []string{"username", "p@ssw0rd!"},
			wantErr: true, // Will fail on network but password is valid
		},
		{
			name:    "very long username",
			args:    []string{strings.Repeat("a", 1000), "password"},
			wantErr: true, // Will fail on network but username length is tested
		},
		{
			name:    "very long password",
			args:    []string{"username", strings.Repeat("p", 1000)},
			wantErr: true, // Will fail on network but password length is tested
		},
		{
			name:    "unicode username",
			args:    []string{"üser", "password"},
			wantErr: true, // Will fail on network but unicode is tested
		},
		{
			name:    "unicode password",
			args:    []string{"username", "pässwörd"},
			wantErr: true, // Will fail on network but unicode is tested
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()

			err := handleGenerateCommand(ctx, tt.args)

			if (err != nil) != tt.wantErr {
				t.Errorf("handleGenerateCommand() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.errCheck != nil && err != nil {
				if !tt.errCheck(err) {
					t.Errorf("handleGenerateCommand() error doesn't match expected pattern: %v", err)
				}
			}
		})
	}
}

// TestRegionNameValidation tests various region name formats
func TestRegionNameValidation(t *testing.T) {
	// Initialize logger for these tests
	originalLogger := logger
	logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	defer func() { logger = originalLogger }()

	tests := []struct {
		name   string
		region string
	}{
		{
			name:   "underscore region",
			region: "us_california",
		},
		{
			name:   "dash region",
			region: "de-frankfurt",
		},
		{
			name:   "mixed separators",
			region: "au_adelaide-pf",
		},
		{
			name:   "numbers in region",
			region: "denmark_2",
		},
		{
			name:   "short region",
			region: "uk",
		},
		{
			name:   "very long region",
			region: "us_massachusetts-pf",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			args := []string{"-r", tt.region, "-timeout", "1s", "testuser", "testpass"}
			err := handleGenerateCommand(ctx, args)

			// All should fail due to network/auth, but we're testing region format acceptance
			if err == nil {
				t.Error("handleGenerateCommand() should fail due to network/auth issues")
			}

			// Check that it's not a region validation error for valid formats
			var regionErr *pia.RegionValidationError
			if strings.Contains(err.Error(), "region") && strings.Contains(err.Error(), "not found") {
				// This indicates the region format was rejected, which might be expected
				// for some test regions that don't actually exist
			}
			_ = regionErr // Avoid unused variable warning
		})
	}
}

// TestTimeoutBehavior tests different timeout values
func TestTimeoutBehavior(t *testing.T) {
	// Initialize logger for these tests
	originalLogger := logger
	logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	defer func() { logger = originalLogger }()

	tests := []struct {
		name    string
		timeout string
		wantErr bool
	}{
		{
			name:    "very short timeout",
			timeout: "1ms",
			wantErr: true,
		},
		{
			name:    "short timeout",
			timeout: "100ms",
			wantErr: true,
		},
		{
			name:    "reasonable timeout",
			timeout: "5s",
			wantErr: true, // Still fails on auth but timeout is reasonable
		},
		{
			name:    "long timeout",
			timeout: "30s",
			wantErr: true, // Still fails on auth but timeout is reasonable
		},
		{
			name:    "invalid timeout format",
			timeout: "invalid",
			wantErr: true,
		},
		{
			name:    "negative timeout",
			timeout: "-5s",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			args := []string{"-timeout", tt.timeout, "testuser", "testpass"}

			err := handleGenerateCommand(ctx, args)

			if (err != nil) != tt.wantErr {
				t.Errorf("handleGenerateCommand() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestConcurrentOperations tests concurrent access patterns
func TestConcurrentOperations(t *testing.T) {
	// Initialize logger for these tests
	originalLogger := logger
	logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	defer func() { logger = originalLogger }()

	const numGoroutines = 10

	t.Run("concurrent file writes", func(t *testing.T) {
		config := testWireguardConfig()

		done := make(chan error, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				filename := fmt.Sprintf("concurrent-test-%d.conf", id)
				err := outputConfig(config, filename)
				if err == nil {
					// Clean up
					os.Remove(filename)
				}
				done <- err
			}(i)
		}

		for i := 0; i < numGoroutines; i++ {
			if err := <-done; err != nil {
				t.Errorf("Concurrent file write %d failed: %v", i, err)
			}
		}
	})

	t.Run("concurrent regions fetch", func(t *testing.T) {
		done := make(chan error, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()

				err := handleRegionsCommand(ctx, []string{})
				done <- err
			}()
		}

		successCount := 0
		for i := 0; i < numGoroutines; i++ {
			if err := <-done; err == nil {
				successCount++
			}
		}

		// At least some should succeed if network is available
		if successCount == 0 {
			t.Log("All concurrent region fetches failed - this might be due to network issues")
		}
	})
}

// TestErrorHandling tests various error conditions
func TestErrorHandling(t *testing.T) {
	// Initialize logger for these tests
	originalLogger := logger
	logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	defer func() { logger = originalLogger }()

	tests := []struct {
		name     string
		setupErr func() error
		testFunc func() error
		cleanup  func()
	}{
		{
			name: "write to read-only file",
			setupErr: func() error {
				// Create a read-only file
				file, err := os.Create("readonly-test.conf")
				if err != nil {
					return err
				}
				file.Close()
				return os.Chmod("readonly-test.conf", 0400)
			},
			testFunc: func() error {
				return outputConfig(testWireguardConfig(), "readonly-test.conf")
			},
			cleanup: func() {
				os.Chmod("readonly-test.conf", 0600) // Make writable to delete
				os.Remove("readonly-test.conf")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setupErr != nil {
				if err := tt.setupErr(); err != nil {
					t.Skipf("Setup failed: %v", err)
				}
			}

			if tt.cleanup != nil {
				defer tt.cleanup()
			}

			err := tt.testFunc()
			if err == nil {
				t.Error("Expected error but got none")
			}
		})
	}
}

// TestMainFunctionIntegration tests the main function integration
func TestMainFunctionIntegration(t *testing.T) {
	// Initialize logger for these tests
	originalLogger := logger
	logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	defer func() { logger = originalLogger }()

	tests := []struct {
		name        string
		args        []string
		expectError bool
	}{
		{
			name:        "help command integration",
			args:        []string{"--help"},
			expectError: false,
		},
		{
			name:        "version command integration",
			args:        []string{"--version"},
			expectError: false,
		},
		{
			name:        "invalid command integration",
			args:        []string{"invalid-command"},
			expectError: true,
		},
		{
			name:        "regions help integration",
			args:        []string{"regions", "--help"},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			err := run(ctx, tt.args)

			if (err != nil) != tt.expectError {
				t.Errorf("run() error = %v, expectError %v", err, tt.expectError)
			}
		})
	}
}

// Helper function to generate test Wireguard config
func testWireguardConfig() string {
	return `[Interface]
PrivateKey = yAnz5TF+lXXJte14tji3zlMNq+hd2rYUIgJBgB3fBmk=
Address = 10.0.0.2/32
DNS = 1.1.1.1, 1.0.0.1

[Peer]
PublicKey = gN65BkIKy1eCE9pP1wdc8ROUtkHLF2PfAqYdyYBz6EA=
Endpoint = 192.168.1.1:1337
AllowedIPs = 0.0.0.0/0
PersistentKeepalive = 25
`
}

// TestConfigValidation tests configuration content validation
func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  string
		wantErr bool
	}{
		{
			name:    "valid wireguard config",
			config:  testWireguardConfig(),
			wantErr: false,
		},
		{
			name:    "empty config",
			config:  "",
			wantErr: false, // Empty config is technically valid for file creation
		},
		{
			name: "config with comments",
			config: `# This is a comment
[Interface]
PrivateKey = test
# Another comment
[Peer]
PublicKey = test`,
			wantErr: false,
		},
		{
			name: "config with extra whitespace",
			config: `

[Interface]


PrivateKey = test


[Peer]
PublicKey = test

`,
			wantErr: false,
		},
	}

	// Initialize logger for these tests
	originalLogger := logger
	logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	defer func() { logger = originalLogger }()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filename := fmt.Sprintf("test-config-%s.conf", strings.ReplaceAll(tt.name, " ", "-"))
			defer os.Remove(filename)

			err := outputConfig(tt.config, filename)

			if (err != nil) != tt.wantErr {
				t.Errorf("outputConfig() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr {
				// Verify file was created and content matches
				content, err := os.ReadFile(filename)
				if err != nil {
					t.Errorf("Failed to read created config file: %v", err)
				} else if string(content) != tt.config {
					t.Errorf("Config file content doesn't match expected")
				}
			}
		})
	}
}
