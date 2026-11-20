package pia

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
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
	username         string
	password         string
	verbose          bool
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

	return &piaClient, nil
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

	if p.verbose {
		log.Print("Got token")
	}

	return tokenResp.Token, nil
}

// GetAvailableRegions returns all online regions that have Wireguard servers
func GetAvailableRegions() (map[Region]string, error) {
	serverList, err := getServerList()
	if err != nil {
		return nil, err
	}

	regions := make(map[Region]string)
	for _, r := range serverList.Regions {
		if r.Offline || len(r.Servers.Wg) == 0 {
			continue
		}
		regions[Region(r.ID)] = r.Name
	}

	return regions, nil
}

// AddKey
func (p *PIAClient) AddKey(token, publickey string) (AddKeyResult, error) {
	var addKeyResp AddKeyResult
	server := p.getWireguardServerForRegion()

	// Build http request
	url := fmt.Sprintf("https://%v:1337/addKey?pt=%v&pubkey=%v", server.Cn, url.QueryEscape(token), url.QueryEscape(publickey))

	// Send request
	resp, err := p.executePIARequest(server, url, token)
	if err != nil {
		return addKeyResp, errors.Wrap(err, "error executing request")
	}

	// Parse response
	err = json.NewDecoder(resp.Body).Decode(&addKeyResp)
	if err != nil {
		return addKeyResp, errors.Wrap(err, "error decoding add key response")
	}

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

func (p *PIAClient) executePIARequest(server Server, url, token string) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	// Set header to JSON
	req.Header.Set("Content-Type", "application/json")

	// Set basic auth
	if token == "" {
		req.SetBasicAuth(p.username, p.password)
	}

	// Add certificate to shared pool
	err = p.downloadPIACertificate()
	if err != nil {
		return nil, errors.Wrap(err, "error downloading ca certificate")
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(p.caCert)

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
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: caCertPool,
			},
			DialContext: dialContext,
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	// Return error if status code is not 200
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("status code %v", resp.StatusCode)
	}

	return resp, nil
}

// downloadPIACertificate downloads the PIA certificate
func (p *PIAClient) downloadPIACertificate() error {
	// caCert already loaded
	if len(p.caCert) > 0 {
		return nil
	}

	// Download certificate
	resp, err := http.Get("https://raw.githubusercontent.com/pia-foss/desktop/master/daemon/res/ca/rsa_4096.crt")
	if err != nil {
		return err
	}

	// Parse certificate
	p.caCert, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	return nil
}
