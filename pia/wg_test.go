package pia

import (
	"testing"
)

type PIAClientMock struct {
	serverPort int
	dnsServers []string
	serverName string
}

func (p *PIAClientMock) GetToken() (string, error) {
	return "", nil
}

func (p *PIAClientMock) AddKey(token, publickey string) (AddKeyResult, error) {
	return AddKeyResult{
		ServerIP:   "1.2.3.4",
		ServerPort: p.serverPort,
		DNSServers: p.dnsServers,
		PeerIP:     "4.5.6.7",
		ServerKey:  publickey,
		ServerName: p.serverName,
	}, nil
}

func TestPIAWgGenerator_Generate(t *testing.T) {
	type fields struct {
		pia        PIAWgClient
		config     PIAWgGeneratorConfig
		verbose    bool
		privatekey string
		publickey  string
	}
	tests := []struct {
		name    string
		fields  fields
		want    string
		wantErr bool
	}{
		{
			name: "basic generate",
			fields: fields{
				pia: &PIAClientMock{serverPort: 1337, dnsServers: []string{"1.1.1.1"}, serverName: "Server-1-0a"},
				config: PIAWgGeneratorConfig{
					Verbose:    false,
					PrivateKey: "test_privatekey",
					PublicKey:  "test_publickey",
				},
			},
			want: `# ServerName: Server-1-0a
[Interface]
PrivateKey = test_privatekey
Address = 4.5.6.7
DNS = 1.1.1.1
[Peer]
PublicKey = test_publickey
AllowedIPs = 0.0.0.0/0
Endpoint = 1.2.3.4:1337
PersistentKeepalive = 25`,
			wantErr: false,
		},
		{
			name: "endpoint uses server port from response",
			fields: fields{
				pia: &PIAClientMock{serverPort: 51820, dnsServers: []string{"1.1.1.1"}},
				config: PIAWgGeneratorConfig{
					PrivateKey: "test_privatekey",
					PublicKey:  "test_publickey",
				},
			},
			want: `[Interface]
PrivateKey = test_privatekey
Address = 4.5.6.7
DNS = 1.1.1.1
[Peer]
PublicKey = test_publickey
AllowedIPs = 0.0.0.0/0
Endpoint = 1.2.3.4:51820
PersistentKeepalive = 25`,
		},
		{
			name: "no DNS servers in response",
			fields: fields{
				pia: &PIAClientMock{serverPort: 1337},
				config: PIAWgGeneratorConfig{
					PrivateKey: "test_privatekey",
					PublicKey:  "test_publickey",
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewPIAWgGenerator(tt.fields.pia, tt.fields.config)
			got, err := p.Generate()
			if (err != nil) != tt.wantErr {
				t.Errorf("PIAWgGenerator.Generate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("PIAWgGenerator.Generate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPIAWgGenerator_generateKeys(t *testing.T) {
	type fields struct {
		pia     PIAWgClient
		verbose bool
	}
	tests := []struct {
		name       string
		fields     fields
		wantResult bool
		wantErr    bool
	}{
		{
			name: "basic generateKeys",
			fields: fields{
				pia: &PIAClientMock{},
			},
			wantResult: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &PIAWgGenerator{
				pia:     tt.fields.pia,
				verbose: tt.fields.verbose,
			}
			got, got1, err := p.generateKeys()
			if (err != nil) != tt.wantErr {
				t.Errorf("PIAWgGenerator.generateKeys() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if (got == "" || got1 == "") && tt.wantResult {
				t.Errorf("PIAWgGenerator.generateKeys() got no keys")
			}
		})
	}
}
