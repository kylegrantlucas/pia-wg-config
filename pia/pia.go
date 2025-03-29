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
	"strings"
	"time"

	"github.com/benburkert/dns"
	"github.com/pkg/errors"
)

type PIAWgClient interface {
	GetToken() (string, error)
	AddKey(token, publickey string) (AddKeyResult, error)
	getMetadataServerForRegion() Server
}

type Region string
type ServerList map[Region][]Server

type PIAClient struct {
	region           string
	wireguardServers ServerList
	metadataServers  ServerList
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
	serverList, err := piaClient.getServerList()
	if err != nil {
		return nil, err
	}

	// Set servers
	piaClient.metadataServers = piaClient.generateMetadataServerList(serverList)
	piaClient.wireguardServers = piaClient.generateWireguardServerList(serverList)

	return &piaClient, nil
}

// GetToken
func (p *PIAClient) GetToken() (string, error) {
	server := p.getMetadataServerForRegion()

	url := fmt.Sprintf("https://%v/authv3/generateToken", server.Cn)

	// Send request
	resp, err := p.executePIARequest(server, url, "")
	if err != nil {
		return "", errors.Wrap(err, "error executing request")
	}

	// Parse response
	var tokenResp struct {
		Token string `json:"token"`
	}

	err = json.NewDecoder(resp.Body).Decode(&tokenResp)
	if err != nil {
		return "", errors.Wrap(err, "error decoding token response")
	}

	if p.verbose {
		log.Print("Got token: ", tokenResp.Token)
	}

	return tokenResp.Token, nil
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
	return p.wireguardServers[Region(p.region)][0]
}

func (p *PIAClient) getMetadataServerForRegion() Server {
	if p.verbose {
		log.Print("Getting metadata server for region: ", p.region)
	}
	return p.metadataServers[Region(p.region)][0]
}

// getSeverList returns a list of servers from the PIA API
func (p *PIAClient) getServerList() (piaServerList, error) {
	var serverList piaServerList

	resp, err := http.Get("https://serverlist.piaservers.net/vpninfo/servers/v6")
	if err != nil {
		return piaServerList{}, err
	}

	// Strip the base64 garbage
	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return piaServerList{}, err
	}
	respString := string(respBytes)
	lastBracketInd := strings.LastIndex(respString, "}")
	safeJSON := respString[:lastBracketInd+1]

	// Parse the JSON
	err = json.Unmarshal([]byte(safeJSON), &serverList)
	if err != nil {
		return piaServerList{}, err
	}

	// Return list of servers
	return serverList, nil
}

// generateWireguardServerList
func (p *PIAClient) generateWireguardServerList(list piaServerList) ServerList {
	servers := ServerList{}

	for _, r := range list.Regions {
		for _, server := range r.Servers.Wg {
			servers[Region(r.ID)] = append(servers[Region(r.Name)], Server{
				Cn: server.Cn,
				IP: server.IP,
			})
		}
	}

	return servers
}

// generateMetadataServerList
func (p *PIAClient) generateMetadataServerList(list piaServerList) ServerList {
	servers := ServerList{}

	for _, r := range list.Regions {
		for _, server := range r.Servers.Meta {
			servers[Region(r.ID)] = append(servers[Region(r.Name)], Server{
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

	// Execute the request
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	// Log the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	resp.Body = io.NopCloser(bytes.NewReader(body))

	// Return error if status code is not 200
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("status code %v, response body: %s", resp.StatusCode, string(body))
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
