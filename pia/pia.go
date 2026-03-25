package pia

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/benburkert/dns"
	"github.com/pkg/errors"
)

// piaCACertFingerprintSHA256 is the expected SHA-256 fingerprint (lowercase hex, no colons) of
// the PIA RSA-4096 CA certificate's DER encoding. This guards against a TOFU/MITM attack on
// the auto-download path by refusing to use a cert that doesn't match this value.
//
// To compute it from a trusted copy of the certificate:
//
//	curl -s https://raw.githubusercontent.com/pia-foss/desktop/master/daemon/res/ca/rsa_4096.crt \
//	  | openssl x509 -noout -fingerprint -sha256 \
//	  | sed 's/.*=//;s/://g' | tr 'A-F' 'a-f'
//
// Cross-verify the result against PIA's official open-source repository:
// https://github.com/pia-foss/desktop/blob/master/daemon/res/ca/rsa_4096.crt
//
// Leave this empty to disable the auto-download path entirely and require --ca-cert.
// This 1fd2... value was correct as of 25 March 2026
const piaCACertFingerprintSHA256 = "1fd25658456eab3041fba77ccd398ab8124edcc1b8b2fc1d55fdf6b1bbfc9d70"

// productionTokenURL is the PIA central API endpoint used to obtain authentication tokens.
// It works with all PIA server generations, including the new Server-XXXXX-0a format.
const productionTokenURL = "https://www.privateinternetaccess.com/api/client/v2/token"

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
	// caCertPath is the path to a local CA cert file. When non-empty it is used
	// in preference to the auto-download path, bypassing any network fetch.
	caCertPath string
	// tokenURL overrides productionTokenURL. Empty means use productionTokenURL.
	// This field exists solely to allow unit tests to point GetToken() at a local httptest server.
	tokenURL string
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
			Wg []Server `json:"wg"`
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

// NewPIAClient creates a new PIA client for with the list of servers populated.
// caCertPath may be empty, in which case the CA cert is downloaded from PIA's GitHub
// repository and verified against the piaCACertFingerprintSHA256 constant.
func NewPIAClient(username, password, region, caCertPath string, verbose bool) (*PIAClient, error) {
	piaClient := PIAClient{
		username:   username,
		password:   password,
		region:     region,
		verbose:    verbose,
		caCertPath: caCertPath,
	}

	// Get list of servers
	serverList, err := piaClient.getServerList()
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch server list from PIA")
	}

	// Set servers
	piaClient.wireguardServers = piaClient.generateWireguardServerList(serverList)

	// Validate region exists
	if _, exists := piaClient.wireguardServers[Region(region)]; !exists {
		availableRegions := make([]string, 0, len(piaClient.wireguardServers))
		for r := range piaClient.wireguardServers {
			availableRegions = append(availableRegions, string(r))
		}
		return nil, fmt.Errorf("region '%s' not found. Available regions: %v. Use 'pia-wg-config regions' to see all available regions", region, availableRegions[:5]) // Show first 5 as example
	}

	// Pre-load the CA certificate so we fail fast if the supplied path is invalid.
	// This is also a no-op cache warm-up on the auto-download path.
	if err := piaClient.downloadPIACertificate(); err != nil {
		return nil, errors.Wrap(err, "loading PIA CA certificate")
	}

	return &piaClient, nil
}

// GetToken fetches an authentication token from PIA's central API.
// The central endpoint works with all server generations, including the new
// Server-XXXXX-0a format whose regional meta servers do not respond to
// the old /authv3/generateToken endpoint.
func (p *PIAClient) GetToken() (string, error) {
	endpoint := productionTokenURL
	if p.tokenURL != "" {
		endpoint = p.tokenURL
	}

	if p.verbose {
		log.Print("Requesting token from PIA central API")
	}

	httpClient := &http.Client{Timeout: 30 * time.Second}
	resp, err := httpClient.PostForm(endpoint, url.Values{
		"username": {p.username},
		"password": {p.password},
	})
	if err != nil {
		return "", errors.Wrap(err, "requesting token from PIA API")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return "", errors.Wrap(err, "reading token response body")
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", errors.Wrap(err, "decoding token response")
	}
	if tokenResp.Token == "" {
		return "", errors.New("received empty token from PIA API")
	}

	if p.verbose {
		log.Printf("Got token: %d bytes", len(tokenResp.Token))
	}

	return tokenResp.Token, nil
}

// GetAvailableRegions returns all available regions
func (p *PIAClient) GetAvailableRegions() (map[Region]string, error) {
	serverList, err := p.getServerList()
	if err != nil {
		return nil, err
	}

	regions := make(map[Region]string)
	for _, r := range serverList.Regions {
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
	resp, err := p.executePIARequest(server, url)
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

// getSeverList returns a list of servers from the PIA API
func (p *PIAClient) getServerList() (piaServerList, error) {
	var serverList piaServerList

	resp, err := http.Get("https://serverlist.piaservers.net/vpninfo/servers/v4")
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
			servers[Region(r.ID)] = append(servers[Region(r.ID)], Server{
				Cn: server.Cn,
				IP: server.IP,
			})
		}
	}

	return servers
}

func (p *PIAClient) executePIARequest(server Server, url string) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	// Set header to JSON
	req.Header.Set("Content-Type", "application/json")

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

// downloadPIACertificate loads the PIA CA certificate.
//
// If PIAClient.caCertPath is set, the cert is read from that file — the caller is
// responsible for obtaining and trusting the file.
//
// Otherwise the cert is fetched from PIA's GitHub repository. The download is
// rejected unless piaCACertFingerprintSHA256 is non-empty AND the SHA-256
// fingerprint of the fetched cert's DER encoding matches it exactly. This
// prevents a MITM attacker from substituting a rogue CA cert.
func (p *PIAClient) downloadPIACertificate() error {
	// caCert already loaded
	if len(p.caCert) > 0 {
		if p.verbose {
			log.Print("CA cert already loaded, skipping download")
		}
		return nil
	}

	// Prefer a locally-provided cert file over the network download.
	if p.caCertPath != "" {
		data, err := os.ReadFile(p.caCertPath)
		if err != nil {
			return fmt.Errorf("reading CA cert from %s: %w", p.caCertPath, err)
		}
		p.caCert = data
		if p.verbose {
			log.Printf("Loaded CA cert from file: %d bytes", len(data))
		}
		return nil
	}

	// Auto-download requires a pinned fingerprint to prevent TOFU attacks.
	if piaCACertFingerprintSHA256 == "" {
		return errors.New(
			"CA cert fingerprint not configured: either supply --ca-cert <path> with a " +
				"locally-trusted cert, or set piaCACertFingerprintSHA256 in pia/pia.go " +
				"after verifying the value with the instructions in that file",
		)
	}

	// Download certificate with a timeout.
	httpClient := &http.Client{Timeout: 30 * time.Second}
	resp, err := httpClient.Get("https://raw.githubusercontent.com/pia-foss/desktop/master/daemon/res/ca/rsa_4096.crt")
	if err != nil {
		return fmt.Errorf("downloading PIA CA cert: %w", err)
	}
	defer resp.Body.Close()

	rawPEM, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024)) // 64 KiB is ample for any cert
	if err != nil {
		return fmt.Errorf("reading PIA CA cert body: %w", err)
	}
	if p.verbose {
		log.Printf("Downloaded CA cert: %d bytes", len(rawPEM))
	}

	// Parse to DER so we can fingerprint the canonical encoding, not the PEM bytes.
	pemBlock, _ := pem.Decode(rawPEM)
	if pemBlock == nil {
		return errors.New("PIA CA cert download: no PEM block found")
	}
	cert, err := x509.ParseCertificate(pemBlock.Bytes)
	if err != nil {
		return fmt.Errorf("PIA CA cert download: parsing certificate: %w", err)
	}

	// Verify fingerprint against the pinned constant.
	fingerprint := sha256.Sum256(cert.Raw)
	got := hex.EncodeToString(fingerprint[:])
	if p.verbose {
		log.Printf("CA cert fingerprint (SHA-256): %s", got)
		log.Printf("CA cert subject: %s", cert.Subject)
		log.Printf("CA cert expires: %s", cert.NotAfter.Format("2006-01-02"))
	}
	if got != piaCACertFingerprintSHA256 {
		return fmt.Errorf(
			"PIA CA cert fingerprint mismatch: pinned=%s got=%s — aborting to prevent MITM",
			piaCACertFingerprintSHA256, got,
		)
	}
	if p.verbose {
		log.Print("CA cert fingerprint verified OK")
	}

	p.caCert = rawPEM
	return nil
}
