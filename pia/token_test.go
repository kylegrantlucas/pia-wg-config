package pia

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newTokenTestClient builds a minimal PIAClient pointed at the given test server URL.
// It bypasses NewPIAClient (which requires live network) by constructing the struct directly.
func newTokenTestClient(serverURL string) *PIAClient {
	return &PIAClient{
		username: "p1234567",
		password: "testpass",
		tokenURL: serverURL,
	}
}

func TestGetToken_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm: %v", err)
		}
		if got := r.FormValue("username"); got != "p1234567" {
			t.Errorf("username: want p1234567, got %s", got)
		}
		if got := r.FormValue("password"); got != "testpass" {
			t.Errorf("password: want testpass, got %s", got)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"token": "abc123token"}) //nolint:errcheck
	}))
	defer srv.Close()

	token, err := newTokenTestClient(srv.URL).GetToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "abc123token" {
		t.Errorf("expected token %q, got %q", "abc123token", token)
	}
}

func TestGetToken_UnauthorizedReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "HTTP Token: Access denied.", http.StatusUnauthorized)
	}))
	defer srv.Close()

	_, err := newTokenTestClient(srv.URL).GetToken()
	if err == nil {
		t.Fatal("expected error for 401, got nil")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("expected 401 in error message, got: %v", err)
	}
}

func TestGetToken_EmptyTokenReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"token": ""}) //nolint:errcheck
	}))
	defer srv.Close()

	_, err := newTokenTestClient(srv.URL).GetToken()
	if err == nil {
		t.Fatal("expected error for empty token, got nil")
	}
	if !strings.Contains(err.Error(), "empty token") {
		t.Errorf("expected 'empty token' in error, got: %v", err)
	}
}

func TestGetToken_MalformedJSONReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("not valid json")) //nolint:errcheck
	}))
	defer srv.Close()

	_, err := newTokenTestClient(srv.URL).GetToken()
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
}
