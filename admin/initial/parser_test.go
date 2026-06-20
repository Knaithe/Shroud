package initial

import (
	"testing"
)

func TestCheckOptions_ValidMinimal(t *testing.T) {
	opt := &Options{
		Secret: []byte("mysecret"),
		Listen: "8080",
	}
	// Reset package-level args so checkOptions doesn't try to resolve
	// addresses from the package-level variable.
	saved := args
	args = opt
	defer func() { args = saved }()

	err := checkOptions(opt)
	if err != nil {
		t.Fatalf("expected no error, got: %s", err.Error())
	}
}

func TestCheckOptions_TLSEnableWithoutFingerprintOrInsecure(t *testing.T) {
	opt := &Options{
		Secret:         []byte("mysecret"),
		Listen:         "8080",
		TlsEnable:      true,
		TlsFingerprint: "",
		TlsInsecure:    false,
	}
	saved := args
	args = opt
	defer func() { args = saved }()

	err := checkOptions(opt)
	if err == nil {
		t.Fatal("expected error when --tls-enable is set without fingerprint or insecure")
	}
	expected := "--tls-enable requires --tls-fingerprint or --tls-insecure"
	if err.Error() != expected {
		t.Fatalf("unexpected error: %s", err.Error())
	}
}

func TestCheckOptions_TLSEnableWithFingerprint(t *testing.T) {
	opt := &Options{
		Secret:         []byte("mysecret"),
		Listen:         "8080",
		TlsEnable:      true,
		TlsFingerprint: "abc123",
		TlsInsecure:    false,
	}
	saved := args
	args = opt
	defer func() { args = saved }()

	err := checkOptions(opt)
	if err != nil {
		t.Fatalf("expected no error with fingerprint, got: %s", err.Error())
	}
}

func TestCheckOptions_TLSEnableWithInsecure(t *testing.T) {
	opt := &Options{
		Secret:         []byte("mysecret"),
		Listen:         "8080",
		TlsEnable:      true,
		TlsFingerprint: "",
		TlsInsecure:    true,
	}
	saved := args
	args = opt
	defer func() { args = saved }()

	err := checkOptions(opt)
	if err != nil {
		t.Fatalf("expected no error with insecure, got: %s", err.Error())
	}
}

func TestCheckOptions_ValidConnect(t *testing.T) {
	opt := &Options{
		Secret:  []byte("mysecret"),
		Connect: "127.0.0.1:9999",
	}
	saved := args
	args = opt
	defer func() { args = saved }()

	err := checkOptions(opt)
	if err != nil {
		t.Fatalf("expected no error, got: %s", err.Error())
	}
}

func TestCheckOptions_InvalidConnectAddr(t *testing.T) {
	opt := &Options{
		Secret:  []byte("mysecret"),
		Connect: "not-valid-addr",
	}
	saved := args
	args = opt
	defer func() { args = saved }()

	err := checkOptions(opt)
	if err == nil {
		t.Fatal("expected error for invalid connect address")
	}
}

func TestCheckOptions_InvalidSocks5Proxy(t *testing.T) {
	opt := &Options{
		Secret:      []byte("mysecret"),
		Connect:     "127.0.0.1:8080",
		Socks5Proxy: "bad-address",
	}
	saved := args
	args = opt
	defer func() { args = saved }()

	err := checkOptions(opt)
	if err == nil {
		t.Fatal("expected error for invalid socks5 proxy address")
	}
}

func TestCheckOptions_ValidSocks5Proxy(t *testing.T) {
	opt := &Options{
		Secret:      []byte("mysecret"),
		Connect:     "127.0.0.1:8080",
		Socks5Proxy: "127.0.0.1:1080",
	}
	saved := args
	args = opt
	defer func() { args = saved }()

	err := checkOptions(opt)
	if err != nil {
		t.Fatalf("expected no error, got: %s", err.Error())
	}
}

func TestCheckOptions_InvalidHttpProxy(t *testing.T) {
	opt := &Options{
		Secret:    []byte("mysecret"),
		Connect:   "127.0.0.1:8080",
		HttpProxy: "bad-proxy",
	}
	saved := args
	args = opt
	defer func() { args = saved }()

	err := checkOptions(opt)
	if err == nil {
		t.Fatal("expected error for invalid http proxy address")
	}
}

func TestCheckOptions_InvalidTorProxy(t *testing.T) {
	opt := &Options{
		Secret:   []byte("mysecret"),
		Connect:  "127.0.0.1:8080",
		TorProxy: "bad-tor",
	}
	saved := args
	args = opt
	defer func() { args = saved }()

	err := checkOptions(opt)
	if err == nil {
		t.Fatal("expected error for invalid tor proxy address")
	}
}

func TestCheckOptions_ValidTorProxy(t *testing.T) {
	opt := &Options{
		Secret:   []byte("mysecret"),
		Connect:  "127.0.0.1:8080",
		TorProxy: "127.0.0.1:9050",
	}
	saved := args
	args = opt
	defer func() { args = saved }()

	err := checkOptions(opt)
	if err != nil {
		t.Fatalf("expected no error, got: %s", err.Error())
	}
}

func TestCheckOptions_TLSDisabledWithFingerprint(t *testing.T) {
	// TLS disabled; fingerprint present but that is fine -- no error expected
	opt := &Options{
		Secret:         []byte("mysecret"),
		Listen:         "8080",
		TlsEnable:      false,
		TlsFingerprint: "somefingerprint",
	}
	saved := args
	args = opt
	defer func() { args = saved }()

	err := checkOptions(opt)
	if err != nil {
		t.Fatalf("expected no error when TLS is disabled, got: %s", err.Error())
	}
}

func TestModeConstants(t *testing.T) {
	if NORMAL_ACTIVE != 0 {
		t.Errorf("NORMAL_ACTIVE expected 0, got %d", NORMAL_ACTIVE)
	}
	if NORMAL_PASSIVE != 1 {
		t.Errorf("NORMAL_PASSIVE expected 1, got %d", NORMAL_PASSIVE)
	}
	if SOCKS5_PROXY_ACTIVE != 2 {
		t.Errorf("SOCKS5_PROXY_ACTIVE expected 2, got %d", SOCKS5_PROXY_ACTIVE)
	}
	if HTTP_PROXY_ACTIVE != 3 {
		t.Errorf("HTTP_PROXY_ACTIVE expected 3, got %d", HTTP_PROXY_ACTIVE)
	}
	if TOR_PROXY_ACTIVE != 4 {
		t.Errorf("TOR_PROXY_ACTIVE expected 4, got %d", TOR_PROXY_ACTIVE)
	}
}
