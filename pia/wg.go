package pia

import (
	"bytes"
	"fmt"
	"log"

	"text/template"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"

	"github.com/pkg/errors"
)

type PIAWgGenerator struct {
	pia        PIAWgClient
	verbose    bool
	serverName bool
	privatekey string
	publickey  string
}

type PIAWgGeneratorConfig struct {
	Verbose    bool
	ServerName bool
	PrivateKey string
	PublicKey  string
}

type templateConfig struct {
	Address             string
	AllowedIPs          string
	DNS                 string
	Endpoint            string
	PrivateKey          string
	PublicKey           string
	PersistentKeepalive string
	ServerCommonName    string
}

func NewPIAWgGenerator(pia PIAWgClient, config PIAWgGeneratorConfig) *PIAWgGenerator {
	return &PIAWgGenerator{
		pia:        pia,
		verbose:    config.Verbose,
		serverName: config.ServerName,
		privatekey: config.PrivateKey,
		publickey:  config.PublicKey,
	}
}

// Generate
func (p *PIAWgGenerator) Generate() (string, error) {
	// Get PIA token
	if p.verbose {
		log.Println("Getting PIA token")
	}
	token, err := p.pia.GetToken()
	if err != nil {
		return "", errors.Wrap(err, "error getting PIA token")
	}

	// Generate Wireguard keys
	if p.verbose {
		log.Println("Generating Wireguard keys")
	}
	privatekey, publickey, err := p.generateKeys()
	if err != nil {
		return "", errors.Wrap(err, "error generating Wireguard keys")
	}

	// Add Wireguard publickey to PIA account
	if p.verbose {
		log.Println("Adding Wireguard publickey to PIA account")
	}
	key, err := p.pia.AddKey(token, publickey)
	if err != nil {
		return "", errors.Wrap(err, "error adding Wireguard publickey to PIA account")
	}

	// Generate Wireguard config
	if p.verbose {
		log.Println("Generating Wireguard config")
	}
	config, err := p.generateConfig(key, privatekey)
	if err != nil {
		return "", errors.Wrap(err, "error generating Wireguard config")
	}

	return config, nil
}

// generateKeys
func (p *PIAWgGenerator) generateKeys() (string, string, error) {
	if p.privatekey != "" && p.publickey != "" {
		return p.privatekey, p.publickey, nil
	}

	privateKey, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		return "", "", errors.Wrap(err, fmt.Sprintf("failed to generate private key: %v", privateKey.String()))
	}
	if p.verbose {
		log.Println("Private key: ", privateKey)
	}

	// Call host 'wg pubkey' to generate public key
	publicKey := privateKey.PublicKey()
	if err != nil {
		return "", "", errors.Wrap(err, fmt.Sprintf("failed to generate public key: %v", publicKey.String()))
	}
	if p.verbose {
		log.Println("Public key: ", publicKey)
	}

	return privateKey.String(), publicKey.String(), nil
}

// generateConfig
func (p *PIAWgGenerator) generateConfig(key AddKeyResult, privatekey string) (string, error) {
	template, err := template.New("config").Parse(wireguardConfigTemplate)
	if err != nil {
		return "", errors.Wrap(err, "error parsing wireguard config template")
	}

	var serverCommonName string
	if p.serverName {
		server := p.pia.getMetadataServerForRegion()
		serverCommonName = server.Cn
	}

	// execute template
	tc := templateConfig{
		PrivateKey:          privatekey,
		PublicKey:           key.ServerKey,
		Endpoint:            key.ServerIP,
		DNS:                 key.DNSServers[0],
		Address:             key.PeerIP,
		AllowedIPs:          "0.0.0.0/0",
		PersistentKeepalive: "25",
		ServerCommonName:    serverCommonName,
	}

	var config bytes.Buffer
	err = template.Execute(&config, tc)
	if err != nil {
		return "", errors.Wrap(err, "error executing wireguard config template")
	}

	return config.String(), nil
}

var wireguardConfigTemplate = `[Interface]
PrivateKey = {{.PrivateKey}}
Address = {{.Address}}
DNS = {{.DNS}}
[Peer]
PublicKey = {{.PublicKey}}
AllowedIPs = {{.AllowedIPs}}
Endpoint = {{.Endpoint}}:1337
PersistentKeepalive = {{.PersistentKeepalive}}
{{- if .ServerCommonName }}
ServerCommonName = {{.ServerCommonName}}
{{- end }}`
