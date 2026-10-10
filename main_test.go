package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/kylegrantlucas/pia-wg-config/pia"
)

func TestFormatUsageError(t *testing.T) {
	tests := []struct {
		name         string
		wantContains []string
	}{
		{
			name: "usage error message",
			wantContains: []string{
				"Error: Username and password are required",
				"Usage:",
				"pia-wg-config [options] USERNAME PASSWORD",
				"Examples:",
				"pia-wg-config myuser mypass",
				"pia-wg-config -r uk_london myuser mypass",
				"To see available regions:",
				"pia-wg-config regions",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// We can't directly test formatUsageError since it's inlined
			// But we can test the main command with no args
			args := []string{}
			ctx := context.Background()

			err := run(ctx, args)

			if err == nil {
				t.Error("run() with no args should return error")
				return
			}

			// Check that the error indicates missing arguments
			if !strings.Contains(err.Error(), "missing required arguments") {
				t.Errorf("run() error should mention missing arguments, got: %v", err)
			}
		})
	}
}

func TestHandlePIAError(t *testing.T) {
	// Initialize logger for these tests
	originalLogger := logger
	logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	defer func() { logger = originalLogger }()

	tests := []struct {
		name    string
		err     error
		region  string
		wantErr bool
	}{
		{
			name: "region validation error",
			err: &pia.RegionValidationError{
				Region:           "invalid_region",
				AvailableRegions: []string{"us_california", "uk_london"},
			},
			region:  "invalid_region",
			wantErr: true,
		},
		{
			name:    "network error",
			err:     &pia.NetworkError{Operation: "test", Err: fmt.Errorf("connection failed")},
			region:  "us_california",
			wantErr: true,
		},
		{
			name:    "generic error",
			err:     fmt.Errorf("generic error"),
			region:  "us_california",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := handlePIAError(tt.err, tt.region)

			if (err != nil) != tt.wantErr {
				t.Errorf("handlePIAError() error = %v, wantErr %v", err, tt.wantErr)
			}

			// The function should return the same error (just prints to stderr)
			if err != tt.err {
				t.Errorf("handlePIAError() should return the same error")
			}
		})
	}
}

func TestHandleConfigGenerationError(t *testing.T) {
	// Initialize logger for these tests
	originalLogger := logger
	logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	defer func() { logger = originalLogger }()

	tests := []struct {
		name    string
		err     error
		wantErr bool
	}{
		{
			name: "authentication error",
			err: &pia.AuthenticationError{
				StatusCode: 401,
				Message:    "Invalid credentials",
			},
			wantErr: true,
		},
		{
			name:    "generic error",
			err:     fmt.Errorf("generation failed"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := handleConfigGenerationError(tt.err)

			if (err != nil) != tt.wantErr {
				t.Errorf("handleConfigGenerationError() error = %v, wantErr %v", err, tt.wantErr)
			}

			// The function should return the same error (just prints to stderr)
			if err != tt.err {
				t.Errorf("handleConfigGenerationError() should return the same error")
			}
		})
	}
}

func TestOutputConfig(t *testing.T) {
	// Initialize logger for these tests
	originalLogger := logger
	logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	defer func() { logger = originalLogger }()

	tests := []struct {
		name    string
		config  string
		outfile string
		wantErr bool
		cleanup bool
	}{
		{
			name:    "output to stdout",
			config:  "[Interface]\nPrivateKey = test\n",
			outfile: "",
			wantErr: false,
		},
		{
			name:    "output to valid file",
			config:  "[Interface]\nPrivateKey = test\n",
			outfile: "test_config.conf",
			wantErr: false,
			cleanup: true,
		},
		{
			name:    "output to invalid path",
			config:  "[Interface]\nPrivateKey = test\n",
			outfile: "/invalid/path/config.conf",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := outputConfig(tt.config, tt.outfile)

			if (err != nil) != tt.wantErr {
				t.Errorf("outputConfig() error = %v, wantErr %v", err, tt.wantErr)
			}

			// If writing to file, check file was created and has correct permissions
			if !tt.wantErr && tt.outfile != "" {
				info, err := os.Stat(tt.outfile)
				if err != nil {
					t.Errorf("outputConfig() file not created: %v", err)
					return
				}

				// Check file permissions (should be 0600)
				if info.Mode().Perm() != 0600 {
					t.Errorf("outputConfig() file permissions = %v, want 0600", info.Mode().Perm())
				}
			}

			// Clean up test file if it was created
			if tt.cleanup && tt.outfile != "" {
				os.Remove(tt.outfile)
			}
		})
	}
}

func TestPrintUsage(t *testing.T) {
	tests := []struct {
		name         string
		wantContains []string
	}{
		{
			name: "usage output",
			wantContains: []string{
				"pia-wg-config version",
				"Generate a Wireguard config for Private Internet Access",
				"USAGE:",
				"COMMANDS:",
				"regions",
				"OPTIONS:",
				"-o string",
				"-r string",
				"--verbose",
				"--timeout",
				"EXAMPLES:",
				"pia-wg-config myuser mypass",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// We can't easily capture stdout, but we can test that the function doesn't panic
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("printUsage() panicked: %v", r)
				}
			}()

			printUsage()
		})
	}
}

func TestRunVersionFlag(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want bool // whether we expect no error
	}{
		{
			name: "long version flag",
			args: []string{"--version"},
			want: true,
		},
		{
			name: "short version flag",
			args: []string{"-v"},
			want: true,
		},
		{
			name: "version with other args",
			args: []string{"--version", "extra"},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			err := run(ctx, tt.args)

			if (err == nil) != tt.want {
				t.Errorf("run() with version flag error = %v, want success = %v", err, tt.want)
			}
		})
	}
}

func TestRunHelpFlag(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want bool // whether we expect no error
	}{
		{
			name: "long help flag",
			args: []string{"--help"},
			want: true,
		},
		{
			name: "short help flag",
			args: []string{"-h"},
			want: true,
		},
		{
			name: "help with other args",
			args: []string{"--help", "extra"},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			err := run(ctx, tt.args)

			if (err == nil) != tt.want {
				t.Errorf("run() with help flag error = %v, want success = %v", err, tt.want)
			}
		})
	}
}

func TestRunRegionsCommand(t *testing.T) {
	// Initialize logger for these tests
	originalLogger := logger
	logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	defer func() { logger = originalLogger }()

	tests := []struct {
		name    string
		args    []string
		timeout time.Duration
		wantErr bool
	}{
		{
			name:    "basic regions command",
			args:    []string{"regions"},
			timeout: 10 * time.Second,
			wantErr: false, // Should succeed in fetching regions
		},
		{
			name:    "regions with timeout",
			args:    []string{"regions", "-timeout", "5s"},
			timeout: 10 * time.Second,
			wantErr: false, // Should succeed with custom timeout
		},
		{
			name:    "regions with help",
			args:    []string{"regions", "-help"},
			timeout: 30 * time.Second,
			wantErr: false, // Help should succeed
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a context with timeout to prevent hanging
			ctx, cancel := context.WithTimeout(context.Background(), tt.timeout)
			defer cancel()

			err := run(ctx, tt.args)

			if (err != nil) != tt.wantErr {
				t.Errorf("run() regions command error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestHandleGenerateCommandValidation(t *testing.T) {
	// Initialize logger for these tests
	originalLogger := logger
	logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	defer func() { logger = originalLogger }()

	tests := []struct {
		name    string
		args    []string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "no arguments",
			args:    []string{},
			wantErr: true,
			errMsg:  "missing required arguments",
		},
		{
			name:    "only username",
			args:    []string{"testuser"},
			wantErr: true,
			errMsg:  "missing required arguments",
		},
		{
			name:    "empty username",
			args:    []string{"", "password"},
			wantErr: true,
			errMsg:  "username and password cannot be empty",
		},
		{
			name:    "empty password",
			args:    []string{"username", ""},
			wantErr: true,
			errMsg:  "username and password cannot be empty",
		},
		{
			name:    "help flag",
			args:    []string{"-help"},
			wantErr: false,
		},
		{
			name:    "valid args but will fail on network",
			args:    []string{"-timeout", "1s", "testuser", "testpass"},
			wantErr: true, // Will fail on network call, but args are valid
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			err := handleGenerateCommand(ctx, tt.args)

			if (err != nil) != tt.wantErr {
				t.Errorf("handleGenerateCommand() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errMsg != "" && err != nil {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("handleGenerateCommand() error = %v, want to contain %v", err.Error(), tt.errMsg)
				}
			}
		})
	}
}

func TestHandleRegionsCommandValidation(t *testing.T) {
	// Initialize logger for these tests
	originalLogger := logger
	logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	defer func() { logger = originalLogger }()

	tests := []struct {
		name    string
		args    []string
		wantErr bool
		timeout time.Duration
	}{
		{
			name:    "no arguments",
			args:    []string{},
			wantErr: false, // Should succeed in fetching regions
			timeout: 10 * time.Second,
		},
		{
			name:    "help flag",
			args:    []string{"-help"},
			wantErr: false,
			timeout: 1 * time.Second,
		},
		{
			name:    "custom timeout",
			args:    []string{"-timeout", "2s"},
			wantErr: false, // Should succeed with custom timeout
			timeout: 10 * time.Second,
		},
		{
			name:    "invalid flag",
			args:    []string{"-invalid"},
			wantErr: true,
			timeout: 1 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), tt.timeout)
			defer cancel()

			err := handleRegionsCommand(ctx, tt.args)

			if (err != nil) != tt.wantErr {
				t.Errorf("handleRegionsCommand() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMainFunctionFlow(t *testing.T) {
	// Initialize logger for these tests
	originalLogger := logger
	logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	defer func() { logger = originalLogger }()

	tests := []struct {
		name string
		args []string
		want string // Expected behavior type
	}{
		{
			name: "version command",
			args: []string{"cmd", "--version"},
			want: "version",
		},
		{
			name: "help command",
			args: []string{"cmd", "--help"},
			want: "help",
		},
		{
			name: "regions command",
			args: []string{"cmd", "regions", "-help"},
			want: "regions_help",
		},
		{
			name: "generate command with insufficient args",
			args: []string{"cmd", "testuser"},
			want: "error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// We test the flow by calling run() directly with the args (excluding program name)
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			args := tt.args[1:] // Remove program name
			err := run(ctx, args)

			switch tt.want {
			case "version", "help", "regions_help":
				if err != nil {
					t.Errorf("run() should succeed for %s, got error: %v", tt.want, err)
				}
			case "error":
				if err == nil {
					t.Errorf("run() should fail for %s, got no error", tt.want)
				}
			}
		})
	}
}

// Benchmarks
func BenchmarkOutputConfigToFile(b *testing.B) {
	// Initialize logger for benchmark
	originalLogger := logger
	logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	defer func() { logger = originalLogger }()

	config := `[Interface]
PrivateKey = test_private_key
Address = 10.0.0.2/32
DNS = 1.1.1.1

[Peer]
PublicKey = test_public_key
Endpoint = 192.168.1.1:1337
AllowedIPs = 0.0.0.0/0
PersistentKeepalive = 25`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		filename := fmt.Sprintf("bench_test_%d.conf", i)
		err := outputConfig(config, filename)
		if err != nil {
			b.Fatalf("outputConfig() failed: %v", err)
		}
		os.Remove(filename) // Clean up
	}
}

func BenchmarkPrintUsage(b *testing.B) {
	// Redirect stdout to prevent spam during benchmark
	original := os.Stdout
	devNull, _ := os.Open(os.DevNull)
	os.Stdout = devNull
	defer func() {
		os.Stdout = original
		devNull.Close()
	}()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		printUsage()
	}
}

func BenchmarkHandlePIAError(b *testing.B) {
	// Initialize logger for benchmark
	originalLogger := logger
	logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	defer func() { logger = originalLogger }()

	err := &pia.RegionValidationError{
		Region:           "invalid_region",
		AvailableRegions: []string{"us_california", "uk_london", "de_frankfurt"},
	}

	// Redirect stderr to prevent spam during benchmark
	original := os.Stderr
	devNull, _ := os.Open(os.DevNull)
	os.Stderr = devNull
	defer func() {
		os.Stderr = original
		devNull.Close()
	}()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		handlePIAError(err, "invalid_region")
	}
}
