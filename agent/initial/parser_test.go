package initial

import (
	"testing"
)

func TestCheckOptions_TLSEnableWithoutFingerprintOrInsecure(t *testing.T) {
	opt := &Options{
		Secret:    []byte("test-secret"),
		TlsEnable: true,
	}
	err := checkOptions(opt)
	if err == nil {
		t.Fatal("expected error when --tls-enable set without fingerprint or insecure")
	}
	if err.Error() != "--tls-enable requires --tls-fingerprint or --tls-insecure" {
		t.Fatalf("unexpected error message: %s", err.Error())
	}
}

func TestCheckOptions_TLSEnableWithFingerprint(t *testing.T) {
	opt := &Options{
		Secret:         []byte("test-secret"),
		TlsEnable:      true,
		TlsFingerprint: "abc123",
	}
	err := checkOptions(opt)
	if err != nil {
		t.Fatalf("unexpected error: %s", err.Error())
	}
}

func TestCheckOptions_TLSEnableWithInsecure(t *testing.T) {
	opt := &Options{
		Secret:      []byte("test-secret"),
		TlsEnable:   true,
		TlsInsecure: true,
	}
	err := checkOptions(opt)
	if err != nil {
		t.Fatalf("unexpected error: %s", err.Error())
	}
}

func TestCheckOptions_ValidConnectAddress(t *testing.T) {
	// checkOptions uses the package-level `args` for Connect/Socks5Proxy/etc checks.
	// Save and restore the global to avoid side effects.
	saved := args
	defer func() { args = saved }()

	args = &Options{
		Secret:  []byte("test-secret"),
		Connect: "127.0.0.1:8080",
	}
	err := checkOptions(args)
	if err != nil {
		t.Fatalf("unexpected error for valid address: %s", err.Error())
	}
}

func TestCheckOptions_InvalidConnectAddress(t *testing.T) {
	saved := args
	defer func() { args = saved }()

	args = &Options{
		Secret:  []byte("test-secret"),
		Connect: "not-a-valid-address",
	}
	err := checkOptions(args)
	if err == nil {
		t.Fatal("expected error for invalid connect address")
	}
}

func TestCheckOptions_ValidSocks5ProxyAddress(t *testing.T) {
	saved := args
	defer func() { args = saved }()

	args = &Options{
		Secret:      []byte("test-secret"),
		Socks5Proxy: "127.0.0.1:1080",
	}
	err := checkOptions(args)
	if err != nil {
		t.Fatalf("unexpected error for valid socks5 proxy: %s", err.Error())
	}
}

func TestCheckOptions_InvalidSocks5ProxyAddress(t *testing.T) {
	saved := args
	defer func() { args = saved }()

	args = &Options{
		Secret:      []byte("test-secret"),
		Socks5Proxy: "bad-addr",
	}
	err := checkOptions(args)
	if err == nil {
		t.Fatal("expected error for invalid socks5 proxy address")
	}
}

func TestCheckOptions_InvalidHttpProxyAddress(t *testing.T) {
	saved := args
	defer func() { args = saved }()

	args = &Options{
		Secret:    []byte("test-secret"),
		HttpProxy: "bad-addr",
	}
	err := checkOptions(args)
	if err == nil {
		t.Fatal("expected error for invalid http proxy address")
	}
}

func TestCheckOptions_InvalidTorProxyAddress(t *testing.T) {
	saved := args
	defer func() { args = saved }()

	args = &Options{
		Secret:   []byte("test-secret"),
		TorProxy: "bad-addr",
	}
	err := checkOptions(args)
	if err == nil {
		t.Fatal("expected error for invalid tor proxy address")
	}
}

func TestCheckOptions_InvalidReuseHost(t *testing.T) {
	saved := args
	defer func() { args = saved }()

	args = &Options{
		Secret:    []byte("test-secret"),
		ReuseHost: "not-an-ip",
	}
	err := checkOptions(args)
	if err == nil {
		t.Fatal("expected error for invalid reuse host")
	}
	if err.Error() != "ReuseHost is not a valid IP addr" {
		t.Fatalf("unexpected error message: %s", err.Error())
	}
}

func TestCheckOptions_ValidReuseHost(t *testing.T) {
	saved := args
	defer func() { args = saved }()

	args = &Options{
		Secret:    []byte("test-secret"),
		ReuseHost: "192.168.1.1",
	}
	err := checkOptions(args)
	if err != nil {
		t.Fatalf("unexpected error for valid reuse host: %s", err.Error())
	}
}

func TestCheckOptions_OnionAddressSkipsResolve(t *testing.T) {
	saved := args
	defer func() { args = saved }()

	// .onion addresses should not be resolved via net.ResolveTCPAddr
	args = &Options{
		Secret:  []byte("test-secret"),
		Connect: "example.onion:80",
	}
	err := checkOptions(args)
	if err != nil {
		t.Fatalf("unexpected error for onion address: %s", err.Error())
	}
}

func TestCheckOptions_TLSDisabledNoError(t *testing.T) {
	opt := &Options{
		Secret:    []byte("test-secret"),
		TlsEnable: false,
		// No fingerprint or insecure needed when TLS is off
	}
	err := checkOptions(opt)
	if err != nil {
		t.Fatalf("unexpected error when TLS is disabled: %s", err.Error())
	}
}
