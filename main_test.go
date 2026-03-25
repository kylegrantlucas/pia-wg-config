package main_test

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// binaryPath is set by TestMain and shared across all tests in this package.
var binaryPath string

// TestMain builds the binary once before running CLI smoke tests.
func TestMain(m *testing.M) {
	bin, err := os.CreateTemp("", "pia-wg-config-*")
	if err != nil {
		panic("failed to create temp file for binary: " + err.Error())
	}
	bin.Close()
	binaryPath = bin.Name()
	defer os.Remove(binaryPath)

	build := exec.Command("go", "build", "-o", binaryPath, ".")
	build.Stdout = os.Stderr
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		panic("failed to build binary: " + err.Error())
	}

	os.Exit(m.Run())
}

// run executes the binary with the given args and returns exit code, stdout, stderr.
func run(t *testing.T, env []string, args ...string) (exitCode int, stdout, stderr string) {
	t.Helper()
	cmd := exec.Command(binaryPath, args...)
	if env != nil {
		cmd.Env = env
	}
	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	if err != nil {
		if ex, ok := err.(*exec.ExitError); ok {
			exitCode = ex.ExitCode()
		} else {
			t.Fatalf("unexpected exec error: %v", err)
		}
	}
	return exitCode, outBuf.String(), errBuf.String()
}

func TestCLI_NoArgs_ExitsNonZero(t *testing.T) {
	code, out, _ := run(t, nil)
	if code == 0 {
		t.Error("expected non-zero exit code with no args")
	}
	if !strings.Contains(out, "username and password") {
		t.Errorf("expected credentials error message, got: %s", out)
	}
}

func TestCLI_NoArgs_ShowsBothCredentialMethods(t *testing.T) {
	_, out, _ := run(t, nil)
	if !strings.Contains(out, "PIAWGCONFIG_USER") {
		t.Error("expected env var instructions in output")
	}
	if !strings.Contains(out, "USERNAME PASSWORD") {
		t.Error("expected positional arg instructions in output")
	}
}

func TestCLI_Help_ExitsZero(t *testing.T) {
	code, out, _ := run(t, nil, "--help")
	if code != 0 {
		t.Errorf("expected exit 0 for --help, got %d", code)
	}
	for _, flag := range []string{"--outfile", "--region", "--verbose", "--ca-cert", "--version"} {
		if !strings.Contains(out, flag) {
			t.Errorf("expected flag %q in help output", flag)
		}
	}
}

func TestCLI_Version_ExitsZeroAndPrintsVersion(t *testing.T) {
	code, out, _ := run(t, nil, "--version")
	if code != 0 {
		t.Errorf("expected exit 0 for --version, got %d", code)
	}
	if !strings.Contains(out, "pia-wg-config") {
		t.Errorf("expected binary name in version output, got: %s", out)
	}
}

func TestCLI_EnvVarCredentials_PassesCredentialCheck(t *testing.T) {
	// Credentials are accepted; the process will fail further on (network/auth),
	// but must NOT fail with the "username and password required" message.
	env := append(os.Environ(), "PIAWGCONFIG_USER=testuser", "PIAWGCONFIG_PW=testpass")
	_, out, _ := run(t, env)
	if strings.Contains(out, "username and password") {
		t.Error("env var credentials should satisfy the credential check")
	}
}

func TestCLI_MissingCACertFile_ReportsError(t *testing.T) {
	code, _, stderr := run(t, nil, "-v", "--ca-cert", "/no/such/file.pem", "p1234567", "pass")
	if code == 0 {
		t.Error("expected non-zero exit with missing ca-cert file")
	}
	if !strings.Contains(stderr, "no such file or directory") {
		t.Errorf("expected file-not-found detail in verbose output, got: %s", stderr)
	}
}

func TestCLI_BadRegion_NamesRegionInError(t *testing.T) {
	// Use a valid-format username so we get past the username check to the region check.
	code, out, _ := run(t, nil, "-r", "BADREGION", "p1234567", "pass")
	if code == 0 {
		t.Error("expected non-zero exit for bad region")
	}
	if !strings.Contains(out, "BADREGION") {
		t.Errorf("expected region name in error output, got: %s", out)
	}
}

func TestCLI_InvalidUsernameFormat_ExitsNonZero(t *testing.T) {
	tests := []string{"user", "alice", "1234", "p", "P", "p123abc", "P123abc"}
	for _, u := range tests {
		code, out, _ := run(t, nil, u, "pass")
		if code == 0 {
			t.Errorf("username %q: expected non-zero exit", u)
		}
		if !strings.Contains(out, "p' followed by digits") {
			t.Errorf("username %q: expected format error message, got: %s", u, out)
		}
	}
}

func TestCLI_ValidUsernameFormat_PassesFormatCheck(t *testing.T) {
	// Both lowercase and uppercase prefix must pass format validation.
	// Will fail further on (network/auth) but must NOT fail on username format.
	for _, u := range []string{"p1234567", "P1234567"} {
		_, out, _ := run(t, nil, u, "pass")
		if strings.Contains(out, "p' followed by digits") {
			t.Errorf("valid username %q should not trigger format error", u)
		}
	}
}

func TestCLI_Regions_ExitsZeroAndListsRegions(t *testing.T) {
	code, out, _ := run(t, nil, "regions")
	if code != 0 {
		t.Skipf("regions command failed (likely no network): exit %d", code)
	}
	if !strings.Contains(out, "us_california") {
		t.Errorf("expected at least us_california in region list, got: %s", out)
	}
	if !strings.Contains(out, "Total:") {
		t.Error("expected Total: line in regions output")
	}
}
