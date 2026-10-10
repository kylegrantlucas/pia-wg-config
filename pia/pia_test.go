package pia

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRegionValidationError_Error(t *testing.T) {
	tests := []struct {
		name   string
		region string
		want   string
	}{
		{
			name:   "basic region error",
			region: "invalid_region",
			want:   "region 'invalid_region' not found",
		},
		{
			name:   "empty region error",
			region: "",
			want:   "region '' not found",
		},
		{
			name:   "special characters",
			region: "test-region_123",
			want:   "region 'test-region_123' not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &RegionValidationError{Region: tt.region}
			if got := err.Error(); got != tt.want {
				t.Errorf("RegionValidationError.Error() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAuthenticationError_Error(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		message    string
		want       string
	}{
		{
			name:       "unauthorized error",
			statusCode: 401,
			message:    "Invalid credentials",
			want:       "authentication failed: Invalid credentials (status: 401)",
		},
		{
			name:       "forbidden error",
			statusCode: 403,
			message:    "Access denied",
			want:       "authentication failed: Access denied (status: 403)",
		},
		{
			name:       "empty message",
			statusCode: 401,
			message:    "",
			want:       "authentication failed:  (status: 401)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &AuthenticationError{
				StatusCode: tt.statusCode,
				Message:    tt.message,
			}
			if got := err.Error(); got != tt.want {
				t.Errorf("AuthenticationError.Error() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNetworkError_Error(t *testing.T) {
	tests := []struct {
		name      string
		operation string
		err       error
		want      string
	}{
		{
			name:      "fetch operation error",
			operation: "fetch server list",
			err:       fmt.Errorf("connection timeout"),
			want:      "network error during fetch server list: connection timeout",
		},
		{
			name:      "authenticate operation error",
			operation: "authenticate",
			err:       fmt.Errorf("host unreachable"),
			want:      "network error during authenticate: host unreachable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &NetworkError{
				Operation: tt.operation,
				Err:       tt.err,
			}
			if got := err.Error(); got != tt.want {
				t.Errorf("NetworkError.Error() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNetworkError_Unwrap(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want error
	}{
		{
			name: "unwrap inner error",
			err:  fmt.Errorf("inner error"),
			want: fmt.Errorf("inner error"),
		},
		{
			name: "unwrap nil error",
			err:  nil,
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			netErr := &NetworkError{
				Operation: "test",
				Err:       tt.err,
			}
			got := netErr.Unwrap()
			if got == nil && tt.want == nil {
				return
			}
			if got == nil || tt.want == nil {
				t.Errorf("NetworkError.Unwrap() = %v, want %v", got, tt.want)
				return
			}
			if got.Error() != tt.want.Error() {
				t.Errorf("NetworkError.Unwrap() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFetchServerList(t *testing.T) {
	tests := []struct {
		name           string
		responseBody   string
		responseStatus int
		wantErr        bool
		wantRegions    int
	}{
		{
			name: "valid server list",
			responseBody: `{
				"regions": [
					{
						"id": "us_california",
						"name": "US California",
						"country": "US",
						"servers": {
							"meta": [{"cn": "meta1.example.com", "ip": "1.2.3.4"}],
							"wg": [{"cn": "wg1.example.com", "ip": "1.2.3.5"}]
						}
					},
					{
						"id": "uk_london",
						"name": "UK London",
						"country": "UK",
						"servers": {
							"meta": [{"cn": "meta2.example.com", "ip": "2.3.4.5"}],
							"wg": [{"cn": "wg2.example.com", "ip": "2.3.4.6"}]
						}
					}
				]
			}`,
			responseStatus: 200,
			wantErr:        false,
			wantRegions:    2,
		},
		{
			name: "server list with base64 garbage",
			responseBody: `{
				"regions": [
					{
						"id": "test_region",
						"name": "Test Region",
						"country": "TEST",
						"servers": {
							"meta": [{"cn": "meta.test.com", "ip": "10.0.0.1"}],
							"wg": [{"cn": "wg.test.com", "ip": "10.0.0.2"}]
						}
					}
				]
			}dGVzdGJhc2U2NGdhcmJhZ2U=`,
			responseStatus: 200,
			wantErr:        false,
			wantRegions:    1,
		},
		{
			name:           "invalid JSON",
			responseBody:   `{"invalid": json}`,
			responseStatus: 200,
			wantErr:        true,
		},
		{
			name:           "server error",
			responseBody:   "",
			responseStatus: 500,
			wantErr:        true,
		},
		{
			name:           "empty response",
			responseBody:   "",
			responseStatus: 200,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.responseStatus)
				w.Write([]byte(tt.responseBody))
			}))
			defer server.Close()

			// We can't easily test fetchServerList directly since it hits a hardcoded URL
			// So we'll test the JSON parsing logic by creating a similar function
			serverList, err := parseServerListJSON([]byte(tt.responseBody))

			if (err != nil) != tt.wantErr {
				t.Errorf("parseServerListJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && len(serverList.Regions) != tt.wantRegions {
				t.Errorf("parseServerListJSON() regions = %d, want %d", len(serverList.Regions), tt.wantRegions)
			}
		})
	}
}

func TestGenerateWireguardServerList(t *testing.T) {
	tests := []struct {
		name        string
		serverList  piaServerList
		wantRegions int
		checkRegion string
		wantServers int
	}{
		{
			name: "basic server list",
			serverList: piaServerList{
				Regions: []struct {
					ID          string `json:"id"`
					Name        string `json:"name"`
					Country     string `json:"country"`
					AutoRegion  bool   `json:"auto_region"`
					DNS         string `json:"dns"`
					PortForward bool   `json:"port_forward"`
					Geo         bool   `json:"geo"`
					Servers     struct {
						Meta []Server `json:"meta"`
						Wg   []Server `json:"wg"`
					} `json:"servers"`
				}{
					{
						ID:   "us_california",
						Name: "US California",
						Servers: struct {
							Meta []Server `json:"meta"`
							Wg   []Server `json:"wg"`
						}{
							Wg: []Server{
								{Cn: "wg1.us.com", IP: "1.2.3.4"},
								{Cn: "wg2.us.com", IP: "1.2.3.5"},
							},
						},
					},
					{
						ID:   "uk_london",
						Name: "UK London",
						Servers: struct {
							Meta []Server `json:"meta"`
							Wg   []Server `json:"wg"`
						}{
							Wg: []Server{
								{Cn: "wg1.uk.com", IP: "2.3.4.5"},
							},
						},
					},
				},
			},
			wantRegions: 2,
			checkRegion: "us_california",
			wantServers: 2,
		},
		{
			name: "empty server list",
			serverList: piaServerList{
				Regions: []struct {
					ID          string `json:"id"`
					Name        string `json:"name"`
					Country     string `json:"country"`
					AutoRegion  bool   `json:"auto_region"`
					DNS         string `json:"dns"`
					PortForward bool   `json:"port_forward"`
					Geo         bool   `json:"geo"`
					Servers     struct {
						Meta []Server `json:"meta"`
						Wg   []Server `json:"wg"`
					} `json:"servers"`
				}{},
			},
			wantRegions: 0,
		},
		{
			name: "region with no WG servers",
			serverList: piaServerList{
				Regions: []struct {
					ID          string `json:"id"`
					Name        string `json:"name"`
					Country     string `json:"country"`
					AutoRegion  bool   `json:"auto_region"`
					DNS         string `json:"dns"`
					PortForward bool   `json:"port_forward"`
					Geo         bool   `json:"geo"`
					Servers     struct {
						Meta []Server `json:"meta"`
						Wg   []Server `json:"wg"`
					} `json:"servers"`
				}{
					{
						ID:   "test_region",
						Name: "Test Region",
						Servers: struct {
							Meta []Server `json:"meta"`
							Wg   []Server `json:"wg"`
						}{
							Wg: []Server{}, // No WG servers
						},
					},
				},
			},
			wantRegions: 0, // Should not add regions without WG servers
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &PIAClient{}
			serverList := client.generateWireguardServerList(tt.serverList)

			if len(serverList) != tt.wantRegions {
				t.Errorf("generateWireguardServerList() regions = %d, want %d", len(serverList), tt.wantRegions)
			}

			if tt.checkRegion != "" {
				servers, exists := serverList[Region(tt.checkRegion)]
				if !exists {
					t.Errorf("generateWireguardServerList() missing region %s", tt.checkRegion)
				} else if len(servers) != tt.wantServers {
					t.Errorf("generateWireguardServerList() region %s has %d servers, want %d",
						tt.checkRegion, len(servers), tt.wantServers)
				}
			}
		})
	}
}

func TestGenerateMetadataServerList(t *testing.T) {
	tests := []struct {
		name        string
		serverList  piaServerList
		wantRegions int
		checkRegion string
		wantServers int
	}{
		{
			name: "basic metadata server list",
			serverList: piaServerList{
				Regions: []struct {
					ID          string `json:"id"`
					Name        string `json:"name"`
					Country     string `json:"country"`
					AutoRegion  bool   `json:"auto_region"`
					DNS         string `json:"dns"`
					PortForward bool   `json:"port_forward"`
					Geo         bool   `json:"geo"`
					Servers     struct {
						Meta []Server `json:"meta"`
						Wg   []Server `json:"wg"`
					} `json:"servers"`
				}{
					{
						ID:   "us_california",
						Name: "US California",
						Servers: struct {
							Meta []Server `json:"meta"`
							Wg   []Server `json:"wg"`
						}{
							Meta: []Server{
								{Cn: "meta1.us.com", IP: "1.2.3.4"},
								{Cn: "meta2.us.com", IP: "1.2.3.5"},
							},
						},
					},
				},
			},
			wantRegions: 1,
			checkRegion: "us_california",
			wantServers: 2,
		},
		{
			name: "region with no meta servers",
			serverList: piaServerList{
				Regions: []struct {
					ID          string `json:"id"`
					Name        string `json:"name"`
					Country     string `json:"country"`
					AutoRegion  bool   `json:"auto_region"`
					DNS         string `json:"dns"`
					PortForward bool   `json:"port_forward"`
					Geo         bool   `json:"geo"`
					Servers     struct {
						Meta []Server `json:"meta"`
						Wg   []Server `json:"wg"`
					} `json:"servers"`
				}{
					{
						ID:   "test_region",
						Name: "Test Region",
						Servers: struct {
							Meta []Server `json:"meta"`
							Wg   []Server `json:"wg"`
						}{
							Meta: []Server{}, // No meta servers
						},
					},
				},
			},
			wantRegions: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &PIAClient{}
			serverList := client.generateMetadataServerList(tt.serverList)

			if len(serverList) != tt.wantRegions {
				t.Errorf("generateMetadataServerList() regions = %d, want %d", len(serverList), tt.wantRegions)
			}

			if tt.checkRegion != "" {
				servers, exists := serverList[Region(tt.checkRegion)]
				if !exists {
					t.Errorf("generateMetadataServerList() missing region %s", tt.checkRegion)
				} else if len(servers) != tt.wantServers {
					t.Errorf("generateMetadataServerList() region %s has %d servers, want %d",
						tt.checkRegion, len(servers), tt.wantServers)
				}
			}
		})
	}
}

func TestValidateRegion(t *testing.T) {
	tests := []struct {
		name        string
		region      string
		serverList  ServerList
		wantErr     bool
		wantErrType string
	}{
		{
			name:   "valid region",
			region: "us_california",
			serverList: ServerList{
				"us_california": []Server{{Cn: "test.com", IP: "1.2.3.4"}},
				"uk_london":     []Server{{Cn: "test2.com", IP: "2.3.4.5"}},
			},
			wantErr: false,
		},
		{
			name:   "invalid region",
			region: "invalid_region",
			serverList: ServerList{
				"us_california": []Server{{Cn: "test.com", IP: "1.2.3.4"}},
				"uk_london":     []Server{{Cn: "test2.com", IP: "2.3.4.5"}},
			},
			wantErr:     true,
			wantErrType: "*pia.RegionValidationError",
		},
		{
			name:        "empty server list",
			region:      "any_region",
			serverList:  ServerList{},
			wantErr:     true,
			wantErrType: "*pia.RegionValidationError",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &PIAClient{
				region:           tt.region,
				wireguardServers: tt.serverList,
			}

			err := client.validateRegion()

			if (err != nil) != tt.wantErr {
				t.Errorf("validateRegion() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.wantErrType != "" {
				errType := fmt.Sprintf("%T", err)
				if errType != tt.wantErrType {
					t.Errorf("validateRegion() error type = %v, want %v", errType, tt.wantErrType)
				}
			}
		})
	}
}

func TestGetWireguardServerForRegion(t *testing.T) {
	tests := []struct {
		name       string
		region     string
		serverList ServerList
		wantPanic  bool
		wantServer Server
	}{
		{
			name:   "valid region with servers",
			region: "us_california",
			serverList: ServerList{
				"us_california": []Server{
					{Cn: "wg1.us.com", IP: "1.2.3.4"},
					{Cn: "wg2.us.com", IP: "1.2.3.5"},
				},
			},
			wantPanic:  false,
			wantServer: Server{Cn: "wg1.us.com", IP: "1.2.3.4"},
		},
		{
			name:       "region with no servers",
			region:     "us_california",
			serverList: ServerList{"us_california": []Server{}},
			wantPanic:  true,
		},
		{
			name:       "missing region",
			region:     "invalid_region",
			serverList: ServerList{"us_california": []Server{{Cn: "test", IP: "1.2.3.4"}}},
			wantPanic:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &PIAClient{
				region:           tt.region,
				wireguardServers: tt.serverList,
				logger:           slog.New(slog.NewTextHandler(nil, &slog.HandlerOptions{Level: slog.LevelError})),
			}

			if tt.wantPanic {
				defer func() {
					if r := recover(); r == nil {
						t.Errorf("getWireguardServerForRegion() did not panic")
					}
				}()
			}

			server := client.getWireguardServerForRegion()

			if !tt.wantPanic {
				if server.Cn != tt.wantServer.Cn || server.IP != tt.wantServer.IP {
					t.Errorf("getWireguardServerForRegion() = %v, want %v", server, tt.wantServer)
				}
			}
		})
	}
}

func TestGetMetadataServerForRegion(t *testing.T) {
	tests := []struct {
		name       string
		region     string
		serverList ServerList
		wantPanic  bool
		wantServer Server
	}{
		{
			name:   "valid region with servers",
			region: "us_california",
			serverList: ServerList{
				"us_california": []Server{
					{Cn: "meta1.us.com", IP: "1.2.3.4"},
					{Cn: "meta2.us.com", IP: "1.2.3.5"},
				},
			},
			wantPanic:  false,
			wantServer: Server{Cn: "meta1.us.com", IP: "1.2.3.4"},
		},
		{
			name:       "region with no servers",
			region:     "us_california",
			serverList: ServerList{"us_california": []Server{}},
			wantPanic:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &PIAClient{
				region:          tt.region,
				metadataServers: tt.serverList,
				logger:          slog.New(slog.NewTextHandler(nil, &slog.HandlerOptions{Level: slog.LevelError})),
			}

			if tt.wantPanic {
				defer func() {
					if r := recover(); r == nil {
						t.Errorf("getMetadataServerForRegion() did not panic")
					}
				}()
			}

			server := client.getMetadataServerForRegion()

			if !tt.wantPanic {
				if server.Cn != tt.wantServer.Cn || server.IP != tt.wantServer.IP {
					t.Errorf("getMetadataServerForRegion() = %v, want %v", server, tt.wantServer)
				}
			}
		})
	}
}

func TestCreateHTTPClient(t *testing.T) {
	tests := []struct {
		name   string
		server Server
		caCert []byte
	}{
		{
			name:   "basic HTTP client creation",
			server: Server{Cn: "test.example.com", IP: "1.2.3.4"},
			caCert: []byte("-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----"),
		},
		{
			name:   "client with different server",
			server: Server{Cn: "another.example.com", IP: "5.6.7.8"},
			caCert: []byte("-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &PIAClient{
				caCert: tt.caCert,
			}

			httpClient := client.createHTTPClient(tt.server)

			if httpClient == nil {
				t.Error("createHTTPClient() returned nil")
				return
			}

			if httpClient.Timeout != 30*time.Second {
				t.Errorf("createHTTPClient() timeout = %v, want %v", httpClient.Timeout, 30*time.Second)
			}

			if httpClient.Transport == nil {
				t.Error("createHTTPClient() transport is nil")
			}
		})
	}
}

// Helper function to test JSON parsing
func parseServerListJSON(data []byte) (piaServerList, error) {
	var serverList piaServerList

	// Strip potential base64 garbage at the end
	dataStr := string(data)
	if lastBracket := strings.LastIndex(dataStr, "}"); lastBracket != -1 {
		dataStr = dataStr[:lastBracket+1]
	}

	if err := json.Unmarshal([]byte(dataStr), &serverList); err != nil {
		return serverList, fmt.Errorf("failed to parse server list JSON: %w", err)
	}

	return serverList, nil
}

// Benchmarks
func BenchmarkGenerateWireguardServerList(b *testing.B) {
	client := &PIAClient{}
	serverList := piaServerList{
		Regions: make([]struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			Country     string `json:"country"`
			AutoRegion  bool   `json:"auto_region"`
			DNS         string `json:"dns"`
			PortForward bool   `json:"port_forward"`
			Geo         bool   `json:"geo"`
			Servers     struct {
				Meta []Server `json:"meta"`
				Wg   []Server `json:"wg"`
			} `json:"servers"`
		}, 100),
	}

	// Fill with test data
	for i := 0; i < 100; i++ {
		serverList.Regions[i] = struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			Country     string `json:"country"`
			AutoRegion  bool   `json:"auto_region"`
			DNS         string `json:"dns"`
			PortForward bool   `json:"port_forward"`
			Geo         bool   `json:"geo"`
			Servers     struct {
				Meta []Server `json:"meta"`
				Wg   []Server `json:"wg"`
			} `json:"servers"`
		}{
			ID:   fmt.Sprintf("region_%d", i),
			Name: fmt.Sprintf("Region %d", i),
			Servers: struct {
				Meta []Server `json:"meta"`
				Wg   []Server `json:"wg"`
			}{
				Wg: []Server{
					{Cn: fmt.Sprintf("wg%d.example.com", i), IP: fmt.Sprintf("1.2.3.%d", i%255)},
				},
			},
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = client.generateWireguardServerList(serverList)
	}
}

func BenchmarkValidateRegion(b *testing.B) {
	serverList := make(ServerList)
	for i := 0; i < 100; i++ {
		serverList[Region(fmt.Sprintf("region_%d", i))] = []Server{
			{Cn: fmt.Sprintf("test%d.com", i), IP: fmt.Sprintf("1.2.3.%d", i%255)},
		}
	}

	client := &PIAClient{
		region:           "region_50",
		wireguardServers: serverList,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = client.validateRegion()
	}
}
