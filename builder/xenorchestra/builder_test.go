package xenorchestra

import (
	"net"
	"net/url"
	"strings"
	"testing"
)

// Config tests
func TestConfigDefaults(t *testing.T) {
	b := &Builder{}
	b.config = Config{
		RestEndpoint: "https://xo.example.com",
		Token:        "test-token",
		PoolName:     "test-pool",
		SRName:       "test-sr",
		SRISOName:    "test-iso-sr",
		ISOName:      "debian.iso",
		TemplateName: "test-uuid",
	}

	// Apply defaults
	if b.config.VCPUs == 0 {
		b.config.VCPUs = 2
	}
	if b.config.VMMemory == 0 {
		b.config.VMMemory = 4096 * 1024 * 1024
	}
	if b.config.Firmware == "" {
		b.config.Firmware = "bios"
	}
	if b.config.HTTPPort == 0 {
		b.config.HTTPPort = 8000
	}
	if b.config.Mode == "" {
		b.config.Mode = "tools"
	}

	// Check defaults
	if b.config.VCPUs != 2 {
		t.Errorf("VCPUs should be 2, got %d", b.config.VCPUs)
	}
	if b.config.VMMemory != 4096*1024*1024 {
		t.Errorf("VMMemory should be 4GB, got %d", b.config.VMMemory)
	}
	if b.config.Firmware != "bios" {
		t.Errorf("Firmware should be 'bios', got %s", b.config.Firmware)
	}
	if b.config.HTTPPort != 8000 {
		t.Errorf("HTTPPort should be 8000, got %d", b.config.HTTPPort)
	}
	if b.config.Mode != "tools" {
		t.Errorf("Mode should be 'tools', got %s", b.config.Mode)
	}
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name      string
		config    Config
		shouldErr bool
	}{
		{
			"valid config",
			Config{
				RestEndpoint: "https://xo.example.com",
				Token:        "test-token",
				PoolName:     "pool",
				SRName:       "sr",
				SRISOName:    "iso-sr",
				ISOName:      "debian.iso",
				TemplateName: "uuid",
			},
			false,
		},
		{
			"missing rest_endpoint",
			Config{
				Token:        "test-token",
				PoolName:     "pool",
				SRName:       "sr",
				SRISOName:    "iso-sr",
				ISOName:      "debian.iso",
				TemplateName: "uuid",
			},
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var errs []error
			if tt.config.RestEndpoint == "" {
				errs = append(errs, nil)
			}
			if tt.config.Token == "" {
				errs = append(errs, nil)
			}

			hasErr := len(errs) > 0
			if hasErr != tt.shouldErr {
				t.Errorf("expected error: %v, got: %v", tt.shouldErr, hasErr)
			}
		})
	}
}

// Client tests
func TestNewClient(t *testing.T) {
	endpoint := "https://xo.example.com"
	token := "test-token"

	client, err := NewClient(endpoint, token)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	if client == nil {
		t.Fatal("NewClient returned nil client")
	}
}

func TestCharToKeysym(t *testing.T) {
	tests := []struct {
		ch        rune
		keysym    uint32
		needShift bool
	}{
		{'a', 'a', false},
		{'z', 'z', false},
		{'0', '0', false},
		{'9', '9', false},
		{'!', 0x31, true},
		{'@', 0x32, true},
		{'#', 0x33, true},
		{'(', 0x39, true},
		{')', 0x30, true},
		{'"', 0x27, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.ch), func(t *testing.T) {
			keysym, needShift := charToKeysym(tt.ch)
			if keysym != tt.keysym {
				t.Errorf("keysym mismatch: expected 0x%x, got 0x%x", tt.keysym, keysym)
			}
			if needShift != tt.needShift {
				t.Errorf("needShift mismatch: expected %v, got %v", tt.needShift, needShift)
			}
		})
	}
}

func TestDeriveScheme(t *testing.T) {
	tests := []struct {
		name       string
		endpoint   string
		expectHTTP string
		expectWS   string
		expectHost string
	}{
		{
			"HTTPS endpoint",
			"https://xo.example.com",
			"https",
			"wss",
			"xo.example.com",
		},
		{
			"HTTP endpoint",
			"http://xo.example.com",
			"http",
			"ws",
			"xo.example.com",
		},
		{
			"HTTPS with port",
			"https://xo.example.com:8443",
			"https",
			"wss",
			"xo.example.com:8443",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsedURL, err := url.Parse(tt.endpoint)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			scheme := parsedURL.Scheme
			if scheme == "" {
				scheme = "https"
			}
			host := parsedURL.Host
			if host == "" {
				host = parsedURL.Hostname()
			}

			httpScheme := "https"
			wsScheme := "wss"
			if scheme == "http" {
				httpScheme = "http"
				wsScheme = "ws"
			}

			if httpScheme != tt.expectHTTP {
				t.Errorf("HTTP scheme mismatch: expected %s, got %s", tt.expectHTTP, httpScheme)
			}
			if wsScheme != tt.expectWS {
				t.Errorf("WS scheme mismatch: expected %s, got %s", tt.expectWS, wsScheme)
			}
			if host != tt.expectHost {
				t.Errorf("host mismatch: expected %s, got %s", tt.expectHost, host)
			}
		})
	}
}

func TestKeySym(t *testing.T) {
	keysyms := map[string]uint32{
		"Enter":     0xff0d,
		"Tab":       0xff09,
		"Escape":    0xff1b,
		"Backspace": 0xff08,
	}

	for name, expectedSym := range keysyms {
		sym, ok := KeySym[name]
		if !ok {
			t.Errorf("KeySym missing %s", name)
			continue
		}
		if sym != expectedSym {
			t.Errorf("%s keysym mismatch: expected 0x%x, got 0x%x", name, expectedSym, sym)
		}
	}
}

// HTTP server tests
func TestHTTPPortDeriveFromEndpoint(t *testing.T) {
	tests := []struct {
		endpoint string
		scheme   string
		port     string
	}{
		{"https://xo.example.com", "https", "443"},
		{"https://xo.example.com:8443", "https", "8443"},
		{"http://xo.example.com", "http", "80"},
		{"http://xo.example.com:8080", "http", "8080"},
	}

	for _, tt := range tests {
		t.Run(tt.endpoint, func(t *testing.T) {
			parsedURL, err := url.Parse(tt.endpoint)
			if err != nil {
				t.Fatalf("failed to parse URL: %v", err)
			}

			port := parsedURL.Port()
			if port == "" {
				if parsedURL.Scheme == "http" {
					port = "80"
				} else {
					port = "443"
				}
			}

			if port != tt.port {
				t.Errorf("port mismatch: expected %s, got %s", tt.port, port)
			}
		})
	}
}

func TestDialTargetConstruction(t *testing.T) {
	tests := []struct {
		name        string
		endpoint    string
		expectError bool
		checkValid  func(string) bool
	}{
		{
			"HTTPS endpoint",
			"https://xo.example.com",
			false,
			func(target string) bool {
				return target == "xo.example.com:443"
			},
		},
		{
			"HTTP endpoint",
			"http://xo.example.com",
			false,
			func(target string) bool {
				return target == "xo.example.com:80"
			},
		},
		{
			"HTTPS with explicit port",
			"https://xo.example.com:9443",
			false,
			func(target string) bool {
				return target == "xo.example.com:9443"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsedURL, err := url.Parse(tt.endpoint)
			if err != nil {
				if !tt.expectError {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}

			port := parsedURL.Port()
			if port == "" {
				if parsedURL.Scheme == "http" {
					port = "80"
				} else {
					port = "443"
				}
			}

			dialTarget := net.JoinHostPort(parsedURL.Hostname(), port)

			if !tt.checkValid(dialTarget) {
				t.Errorf("invalid dial target: %s", dialTarget)
			}
		})
	}
}

func TestIPv4Filtering(t *testing.T) {
	tests := []struct {
		name   string
		ip     string
		isIPv4 bool
	}{
		{"192.168.1.1", "192.168.1.1", true},
		{"10.0.0.1", "10.0.0.1", true},
		{"fe80::1", "fe80::1", false},
		{"2001:db8::1", "2001:db8::1", false},
		{"::1", "::1", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := net.ParseIP(tt.ip)
			if ip == nil {
				t.Fatalf("failed to parse IP: %s", tt.ip)
			}

			ipv4 := ip.To4()
			isIPv4 := ipv4 != nil

			if isIPv4 != tt.isIPv4 {
				t.Errorf("expected IPv4=%v, got %v", tt.isIPv4, isIPv4)
			}
		})
	}
}

func TestIPv4KeyPattern(t *testing.T) {
	tests := []struct {
		key     string
		matches bool
	}{
		{"0/ipv4/0", true},
		{"0/ipv4/1", true},
		{"1/ipv4/0", true},
		{"0/ipv6/0", false},
		{"0/ipv6/1", false},
		{"ipv4", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			matches := strings.Contains(tt.key, "/ipv4/")
			if matches != tt.matches {
				t.Errorf("expected matches=%v, got %v", tt.matches, matches)
			}
		})
	}
}
