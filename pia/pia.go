package pia

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"
)

// Custom error types for better error handling
type RegionValidationError struct {
	Region           string
	AvailableRegions []string
}

func (e *RegionValidationError) Error() string {
	return fmt.Sprintf("region '%s' not found", e.Region)
}

type AuthenticationError struct {
	StatusCode int
	Message    string
}

func (e *AuthenticationError) Error() string {
	return fmt.Sprintf("authentication failed: %s (status: %d)", e.Message, e.StatusCode)
}

type NetworkError struct {
	Operation string
	Err       error
}

func (e *NetworkError) Error() string {
	return fmt.Sprintf("network error during %s: %v", e.Operation, e.Err)
}

func (e *NetworkError) Unwrap() error {
	return e.Err
}

// Types
type Region string

type ServerList map[Region][]Server

type PIAClient struct {
	region           string
	wireguardServers ServerList
	metadataServers  ServerList
	username         string
	password         string
	logger           *slog.Logger
	httpClient       *http.Client
	caCert           []byte
}

type piaServerList struct {
	Regions []struct {
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
	} `json:"regions"`
}

type AddKeyResult struct {
	Status     string   `json:"status"`
	ServerKey  string   `json:"server_key"`
	ServerPort int      `json:"server_port"`
	ServerIP   string   `json:"server_ip"`
	ServerVip  string   `json:"server_vip"`
	PeerIP     string   `json:"peer_ip"`
	PeerPubkey string   `json:"peer_pubkey"`
	DNSServers []string `json:"dns_servers"`
}

type Server struct {
	Cn string `json:"cn"`
	IP string `json:"ip"`
}

// NewPIAClient creates a new PIA client with modern Go patterns
func NewPIAClient(ctx context.Context, username, password, region string, logger *slog.Logger) (*PIAClient, error) {
	if logger == nil {
		logger = slog.Default()
	}

	client := &PIAClient{
		username: username,
		password: password,
		region:   region,
		logger:   logger.With("component", "pia-client"),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	// Get and validate server list
	if err := client.initializeServers(ctx); err != nil {
		return nil, err
	}

	return client, nil
}

// GetAvailableRegions returns all available regions without requiring authentication
func GetAvailableRegions(ctx context.Context, logger *slog.Logger) (map[Region]string, error) {
	if logger == nil {
		logger = slog.Default()
	}

	logger.Debug("Fetching available regions from PIA")

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	serverList, err := fetchServerList(ctx, client, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch server list: %w", err)
	}

	regions := make(map[Region]string, len(serverList.Regions))
	for _, r := range serverList.Regions {
		regions[Region(r.ID)] = r.Name
	}

	logger.Debug("Successfully fetched regions", "count", len(regions))
	return regions, nil
}

// GenerateConfig generates a Wireguard configuration
func (c *PIAClient) GenerateConfig(ctx context.Context) (string, error) {
	c.logger.Debug("Starting Wireguard config generation")

	// Generate token
	token, err := c.getToken(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get authentication token: %w", err)
	}

	// Generate Wireguard key pair
	generator := NewWgGenerator(c, c.logger)
	config, err := generator.Generate(ctx, token)
	if err != nil {
		return "", fmt.Errorf("failed to generate Wireguard config: %w", err)
	}

	c.logger.Info("Successfully generated Wireguard config")
	return config, nil
}

// initializeServers fetches and validates server list
func (c *PIAClient) initializeServers(ctx context.Context) error {
	c.logger.Debug("Initializing server list", "region", c.region)

	serverList, err := fetchServerList(ctx, c.httpClient, c.logger)
	if err != nil {
		return &NetworkError{Operation: "fetch server list", Err: err}
	}

	// Generate server maps
	c.metadataServers = c.generateMetadataServerList(serverList)
	c.wireguardServers = c.generateWireguardServerList(serverList)

	// Validate region exists
	if err := c.validateRegion(); err != nil {
		return err
	}

	c.logger.Debug("Successfully initialized servers",
		"wg_servers", len(c.wireguardServers[Region(c.region)]),
		"meta_servers", len(c.metadataServers[Region(c.region)]))

	return nil
}

// validateRegion checks if the specified region is available
func (c *PIAClient) validateRegion() error {
	if _, exists := c.wireguardServers[Region(c.region)]; !exists {
		availableRegions := make([]string, 0, len(c.wireguardServers))
		for r := range c.wireguardServers {
			availableRegions = append(availableRegions, string(r))
		}
		slices.Sort(availableRegions)

		return &RegionValidationError{
			Region:           c.region,
			AvailableRegions: availableRegions,
		}
	}
	return nil
}

// getToken retrieves authentication token from PIA
func (c *PIAClient) getToken(ctx context.Context) (string, error) {
	c.logger.Debug("Requesting authentication token")

	server := c.getMetadataServerForRegion()
	url := fmt.Sprintf("https://%s/authv3/generateToken", server.Cn)

	resp, err := c.executePIARequest(ctx, server, url, "")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var tokenResp struct {
		Token string `json:"token"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("failed to decode token response: %w", err)
	}

	if tokenResp.Token == "" {
		return "", fmt.Errorf("received empty token from server")
	}

	c.logger.Debug("Successfully obtained authentication token")
	return tokenResp.Token, nil
}

// AddKey adds a public key to PIA for Wireguard
func (c *PIAClient) AddKey(ctx context.Context, token, publickey string) (AddKeyResult, error) {
	c.logger.Debug("Adding public key to PIA")

	var result AddKeyResult
	server := c.getWireguardServerForRegion()

	url := fmt.Sprintf("https://%s:1337/addKey?pt=%s&pubkey=%s",
		server.Cn,
		url.QueryEscape(token),
		url.QueryEscape(publickey))

	resp, err := c.executePIARequest(ctx, server, url, token)
	if err != nil {
		return result, err
	}
	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return result, fmt.Errorf("failed to decode add key response: %w", err)
	}

	c.logger.Debug("Successfully added public key", "server_ip", result.ServerIP)
	return result, nil
}

// getWireguardServerForRegion returns a Wireguard server for the specified region
func (c *PIAClient) getWireguardServerForRegion() Server {
	servers := c.wireguardServers[Region(c.region)]
	if len(servers) == 0 {
		c.logger.Error("No Wireguard servers available for region", "region", c.region)
		panic(fmt.Sprintf("no Wireguard servers available for region: %s", c.region))
	}
	return servers[0]
}

// getMetadataServerForRegion returns a metadata server for the specified region
func (c *PIAClient) getMetadataServerForRegion() Server {
	servers := c.metadataServers[Region(c.region)]
	if len(servers) == 0 {
		c.logger.Error("No metadata servers available for region", "region", c.region)
		panic(fmt.Sprintf("no metadata servers available for region: %s", c.region))
	}
	return servers[0]
}

// fetchServerList retrieves the server list from PIA
func fetchServerList(ctx context.Context, client *http.Client, logger *slog.Logger) (piaServerList, error) {
	var serverList piaServerList

	req, err := http.NewRequestWithContext(ctx, "GET", "https://serverlist.piaservers.net/vpninfo/servers/v4", nil)
	if err != nil {
		return serverList, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return serverList, fmt.Errorf("failed to fetch server list: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return serverList, fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return serverList, fmt.Errorf("failed to read response: %w", err)
	}

	// Strip the base64 garbage at the end
	respString := string(respBytes)
	if lastBracket := strings.LastIndex(respString, "}"); lastBracket != -1 {
		respString = respString[:lastBracket+1]
	}

	if err := json.Unmarshal([]byte(respString), &serverList); err != nil {
		return serverList, fmt.Errorf("failed to parse server list JSON: %w", err)
	}

	logger.Debug("Successfully fetched server list", "regions", len(serverList.Regions))
	return serverList, nil
}

// generateWireguardServerList creates a map of regions to Wireguard servers
func (c *PIAClient) generateWireguardServerList(list piaServerList) ServerList {
	servers := make(ServerList)

	for _, r := range list.Regions {
		if len(r.Servers.Wg) > 0 {
			for _, server := range r.Servers.Wg {
				servers[Region(r.ID)] = append(servers[Region(r.ID)], Server{
					Cn: server.Cn,
					IP: server.IP,
				})
			}
		}
	}

	return servers
}

// generateMetadataServerList creates a map of regions to metadata servers
func (c *PIAClient) generateMetadataServerList(list piaServerList) ServerList {
	servers := make(ServerList)

	for _, r := range list.Regions {
		if len(r.Servers.Meta) > 0 {
			for _, server := range r.Servers.Meta {
				servers[Region(r.ID)] = append(servers[Region(r.ID)], Server{
					Cn: server.Cn,
					IP: server.IP,
				})
			}
		}
	}

	return servers
}

// executePIARequest executes an HTTP request to PIA servers with proper DNS resolution
func (c *PIAClient) executePIARequest(ctx context.Context, server Server, requestURL, token string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Set authentication
	if token == "" {
		req.SetBasicAuth(c.username, c.password)
	}

	// Ensure we have the PIA certificate
	if err := c.downloadPIACertificate(ctx); err != nil {
		return nil, fmt.Errorf("failed to download CA certificate: %w", err)
	}

	// Create HTTP client with custom DNS and TLS config
	client := c.createHTTPClient(server)

	resp, err := client.Do(req)
	if err != nil {
		return nil, &NetworkError{Operation: "execute request", Err: err}
	}

	// Check for authentication errors
	if resp.StatusCode == http.StatusUnauthorized {
		resp.Body.Close()
		return nil, &AuthenticationError{
			StatusCode: resp.StatusCode,
			Message:    "Invalid username or password",
		}
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	return resp, nil
}

// createHTTPClient creates an HTTP client with custom hostname/IP mapping and TLS config
func (c *PIAClient) createHTTPClient(server Server) *http.Client {
	// Create certificate pool
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(c.caCert)

	// Create custom dialer that maps hostname to IP
	dialer := &net.Dialer{
		Timeout: 10 * time.Second,
	}

	// Custom dial function that maps server hostname to IP
	customDialContext := func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}

		// If this is our PIA server hostname, use the known IP
		if host == server.Cn {
			addr = net.JoinHostPort(server.IP, port)
		}

		return dialer.DialContext(ctx, network, addr)
	}

	return &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: caCertPool,
			},
			DialContext: customDialContext,
		},
	}
}

// downloadPIACertificate downloads the PIA CA certificate
func (c *PIAClient) downloadPIACertificate(ctx context.Context) error {
	// Certificate already loaded
	if len(c.caCert) > 0 {
		return nil
	}

	c.logger.Debug("Downloading PIA CA certificate")

	req, err := http.NewRequestWithContext(ctx, "GET",
		"https://raw.githubusercontent.com/pia-foss/desktop/master/daemon/res/ca/rsa_4096.crt", nil)
	if err != nil {
		return fmt.Errorf("failed to create certificate request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download certificate: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download certificate, status: %d", resp.StatusCode)
	}

	c.caCert, err = io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read certificate: %w", err)
	}

	c.logger.Debug("Successfully downloaded PIA CA certificate")
	return nil
}
