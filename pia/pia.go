package pia

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/benburkert/dns"
	"github.com/pkg/errors"
)

type PIAWgClient interface {
	GetToken() (string, error)
	AddKey(token, publickey string) (AddKeyResult, error)
}

type Region string
type ServerList map[Region][]Server

type PIAClient struct {
	region           string
	wireguardServers ServerList
	portForward      bool
	username         string
	password         string
	verbose          bool
}

// piaCACert is PIA's RSA-4096 CA (ca.rsa.4096.crt from pia-foss/manual-connections),
// which signs the certificates of the Wireguard API on port 1337.
//
//go:embed ca.rsa.4096.crt
var piaCACert []byte

type piaServerList struct {
	Regions []struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Country     string `json:"country"`
		AutoRegion  bool   `json:"auto_region"`
		DNS         string `json:"dns"`
		PortForward bool   `json:"port_forward"`
		Geo         bool   `json:"geo"`
		Offline     bool   `json:"offline"`
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

	// Region and ServerName identify the Wireguard server the key was
	// registered with; they are not part of the API response. ServerName is
	// its TLS server name (CN), which PIA's port forwarding API on that
	// server is verified against.
	Region     string `json:"-"`
	ServerName string `json:"-"`
}

type Server struct {
	Cn string
	IP string
}

// NewPIAClient creates a new PIA client for with the list of servers populated
func NewPIAClient(username, password, region string, verbose bool) (*PIAClient, error) {
	piaClient := PIAClient{
		username: username,
		password: password,
		region:   region,
		verbose:  verbose,
	}

	// Get list of servers
	serverList, err := getServerList()
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch server list from PIA")
	}

	// Set servers
	piaClient.wireguardServers = piaClient.generateWireguardServerList(serverList)

	// Validate region exists
	if _, exists := piaClient.wireguardServers[Region(region)]; !exists {
		return nil, fmt.Errorf("region '%s' not found or has no online Wireguard servers. Use 'pia-wg-config regions' to see all available regions", region)
	}

	for _, r := range serverList.Regions {
		if r.ID == region {
			piaClient.portForward = r.PortForward
			break
		}
	}

	return &piaClient, nil
}

// SupportsPortForwarding reports whether the client's region supports port forwarding
func (p *PIAClient) SupportsPortForwarding() bool {
	return p.portForward
}

// tokenURL is PIA's central token API (the same endpoint used by PIA's
// manual-connections scripts). Regional metadata servers no longer serve
// /authv3/generateToken reliably (issues #12, #17).
const tokenURL = "https://www.privateinternetaccess.com/api/client/v2/token"

// GetToken
func (p *PIAClient) GetToken() (string, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.PostForm(tokenURL, url.Values{
		"username": {p.username},
		"password": {p.password},
	})
	if err != nil {
		return "", errors.Wrap(err, "error executing request")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token request failed: %s", resp.Status)
	}
	// Parse response
	var tokenResp struct {
		Token string `json:"token"`
	}

	err = json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&tokenResp)
	if err != nil {
		return "", errors.Wrap(err, "error decoding token response")
	}
	if tokenResp.Token == "" {
		return "", errors.New("token response contained no token")
	}

	if p.verbose {
		log.Print("Got token")
	}

	return tokenResp.Token, nil
}

// RegionInfo describes a region from PIA's server list
type RegionInfo struct {
	Name        string
	PortForward bool
}

// GetAvailableRegions returns all online regions that have Wireguard servers
func GetAvailableRegions() (map[Region]RegionInfo, error) {
	serverList, err := getServerList()
	if err != nil {
		return nil, err
	}

	regions := make(map[Region]RegionInfo)
	for _, r := range serverList.Regions {
		if r.Offline || len(r.Servers.Wg) == 0 {
			continue
		}
		regions[Region(r.ID)] = RegionInfo{Name: r.Name, PortForward: r.PortForward}
	}

	return regions, nil
}

// AddKey registers publickey with the region's Wireguard server, like
// connect_to_wireguard_with_token.sh in PIA's manual-connections scripts.
func (p *PIAClient) AddKey(token, publickey string) (AddKeyResult, error) {
	var addKeyResp AddKeyResult
	server := p.getWireguardServerForRegion()

	// Build http request
	query := url.Values{"pt": {token}, "pubkey": {publickey}}
	reqURL := fmt.Sprintf("https://%s:1337/addKey?%s", server.Cn, query.Encode())

	// Send request
	resp, err := p.executePIARequest(server, reqURL)
	if err != nil {
		return addKeyResp, errors.Wrap(err, "error executing request")
	}
	defer resp.Body.Close()

	// Parse response
	err = json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&addKeyResp)
	if err != nil {
		return addKeyResp, errors.Wrap(err, "error decoding add key response")
	}
	if addKeyResp.Status != "OK" {
		return addKeyResp, fmt.Errorf("server rejected key: status %q", addKeyResp.Status)
	}
	if addKeyResp.ServerKey == "" || addKeyResp.ServerIP == "" || addKeyResp.PeerIP == "" ||
		addKeyResp.ServerPort <= 0 || addKeyResp.ServerPort > 65535 {
		return addKeyResp, errors.New("add key response is missing server key, server IP, server port or peer IP")
	}
	addKeyResp.Region = p.region
	addKeyResp.ServerName = server.Cn

	return addKeyResp, nil
}

func (p *PIAClient) getWireguardServerForRegion() Server {
	if p.verbose {
		log.Print("Getting wireguard server for region: ", p.region)
	}
	servers := p.wireguardServers[Region(p.region)]
	if len(servers) == 0 {
		log.Fatalf("No Wireguard servers available for region: %s", p.region)
	}
	if p.verbose {
		log.Printf("Using wireguard server %s (%s)", servers[0].Cn, servers[0].IP)
	}
	return servers[0]
}

// serverListURL is the server list used by PIA's manual-connections scripts
// (get_region.sh). The first line of the response is the JSON document; the
// remaining lines are a signature.
const serverListURL = "https://serverlist.piaservers.net/vpninfo/servers/v6"

// getServerList returns a list of servers from the PIA API
func getServerList() (piaServerList, error) {
	var serverList piaServerList

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(serverListURL)
	if err != nil {
		return piaServerList{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return piaServerList{}, fmt.Errorf("server list request failed: %s", resp.Status)
	}

	respBytes, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return piaServerList{}, err
	}

	// Keep only the JSON on the first line, drop the trailing signature
	jsonBytes, _, _ := bytes.Cut(respBytes, []byte("\n"))

	// Parse the JSON
	err = json.Unmarshal(jsonBytes, &serverList)
	if err != nil {
		return piaServerList{}, errors.Wrap(err, "error decoding server list")
	}

	// Return list of servers
	return serverList, nil
}

// generateWireguardServerList
func (p *PIAClient) generateWireguardServerList(list piaServerList) ServerList {
	servers := ServerList{}

	for _, r := range list.Regions {
		if r.Offline {
			continue
		}
		for _, server := range r.Servers.Wg {
			servers[Region(r.ID)] = append(servers[Region(r.ID)], Server{
				Cn: server.Cn,
				IP: server.IP,
			})
		}
	}

	return servers
}

func (p *PIAClient) executePIARequest(server Server, reqURL string) (*http.Response, error) {
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, err
	}

	// Set header to JSON
	req.Header.Set("Content-Type", "application/json")

	// Trust only the PIA CA for the Wireguard API
	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(piaCACert) {
		return nil, errors.New("error loading embedded PIA CA certificate")
	}

	// Create DNS resolver for PIA addresses
	zone := &dns.Zone{
		Origin: "",
		TTL:    5 * time.Minute,
		RRs: dns.RRSet{
			server.Cn: {
				dns.TypeA: []dns.Record{
					&dns.A{A: net.ParseIP(server.IP)},
				},
			},
		},
	}
	mux := new(dns.ResolveMux)
	mux.Handle(dns.TypeANY, zone.Origin, zone)
	resolver := &net.Resolver{
		PreferGo: true,
		Dial: (&dns.Client{
			Resolver: mux,
		}).Dial,
	}

	// Set custom DNS server
	dialer := &net.Dialer{
		Resolver: resolver,
	}

	dialContext := func(ctx context.Context, network, addr string) (net.Conn, error) {
		return dialer.DialContext(ctx, network, addr)
	}

	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: caCertPool,
			},
			DialContext: dialContext,
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		// Drop the request URL from the error: its query string holds the token.
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			err = fmt.Errorf("%s %s (%s): %w", urlErr.Op, server.Cn, server.IP, urlErr.Err)
		}
		return nil, err
	}

	// Return error if status code is not 200
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("unexpected response from %s (%s): %s", server.Cn, server.IP, resp.Status)
	}

	return resp, nil
}
