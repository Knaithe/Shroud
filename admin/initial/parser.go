package initial

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"strings"

	"Shroud/admin/printer"
	"Shroud/protocol"
	"Shroud/share"
	"Shroud/utils"
)

const (
	NORMAL_ACTIVE = iota
	NORMAL_PASSIVE
	SOCKS5_PROXY_ACTIVE
	HTTP_PROXY_ACTIVE
	TOR_PROXY_ACTIVE
)

type Options struct {
	Mode           uint8
	Secret         []byte
	Listen         string
	Connect        string
	Socks5Proxy    string
	Socks5ProxyU   string
	Socks5ProxyP   string
	HttpProxy      string
	TorProxy       string
	Downstream     string
	Domain         string
	TlsEnable      bool
	TlsFingerprint string
	TlsInsecure    bool
	Heartbeat      bool
	Script         bool
	Daemon         bool
	Magic          string
	WSPath         string
	IdentityDir    string
	Passphrase     string
	IdentityPlain  bool
	CAFile         string
	PadSize        int
}

var args *Options
var secretFlag string

var ExitCleanup func() = func() {}

func init() {
	args = new(Options)

	flag.StringVar(&secretFlag, "s", "", "One-time enrollment secret")
	flag.StringVar(&args.Listen, "l", "", "Listen port")
	flag.StringVar(&args.Connect, "c", "", "The node address when you actively connect to it")
	flag.StringVar(&args.Socks5Proxy, "socks5-proxy", "", "The socks5 server ip:port you want to use")
	flag.StringVar(&args.Socks5ProxyU, "socks5-proxyu", "", "socks5 username")
	flag.StringVar(&args.Socks5ProxyP, "socks5-proxyp", "", "socks5 password")
	flag.StringVar(&args.HttpProxy, "http-proxy", "", "The http proxy server ip:port you want to use")
	flag.StringVar(&args.TorProxy, "tor-proxy", "", "The Tor SOCKS5 proxy address (e.g. 127.0.0.1:9050)")
	flag.StringVar(&args.Downstream, "down", "raw", "Downstream data type you want to use")
	flag.StringVar(&args.Domain, "domain", "", "Domain name for TLS SNI/WS")
	flag.BoolVar(&args.TlsEnable, "tls-enable", false, "Encrypt connection by TLS")
	flag.BoolVar(&args.Heartbeat, "heartbeat", false, "Send heartbeat packet to first agent")
	flag.StringVar(&args.TlsFingerprint, "tls-fingerprint", "", "Expected TLS certificate SHA256 fingerprint for pinning")
	flag.BoolVar(&args.TlsInsecure, "tls-insecure", false, "Allow TLS without certificate pinning (TOFU mode)")
	flag.BoolVar(&args.Script, "script", false, "Read commands from stdin instead of interactive terminal")
	flag.BoolVar(&args.Daemon, "daemon", false, "Run as headless daemon (no interactive terminal)")
	flag.StringVar(&args.Magic, "magic", "", "4-byte preauth magic override")
	flag.StringVar(&args.WSPath, "ws-path", "", "WebSocket path override")
	flag.StringVar(&args.IdentityDir, "identity-dir", "", "Identity storage directory")
	flag.StringVar(&args.Passphrase, "passphrase", "", "Passphrase for encrypting identity files")
	flag.BoolVar(&args.IdentityPlain, "identity-plain", false, "Allow plaintext identity storage (unsafe)")
	flag.StringVar(&args.CAFile, "ca-file", "", "Separate CA key file (offline CA)")
	flag.IntVar(&args.PadSize, "pad-size", 0, "Traffic padding block size")

	flag.Usage = newUsage
}

func newUsage() {
	ExitCleanup()

	fmt.Fprintf(os.Stderr, `
Usages:
	>> ./shroud_admin -l <port> -s [secret]
	>> ./shroud_admin -c <ip:port> -s [secret]
	>> ./shroud_admin -c <ip:port> -s [secret] --socks5-proxy <ip:port> --socks5-proxyu [username] --socks5-proxyp [password]
`)
	flag.PrintDefaults()
}

func resolveSecret() []byte {
	if envSec := os.Getenv("SHROUD_SECRET"); envSec != "" {
		os.Unsetenv("SHROUD_SECRET")
		return []byte(envSec)
	}
	if secretFlag != "" {
		sec := []byte(secretFlag)
		secretFlag = ""
		return sec
	}
	return nil
}

// ParseOptions Parsing user's options
func ParseOptions() *Options {
	flag.Parse()
	args.Secret = resolveSecret()
	scrubSecretArgs()
	utils.ScrubCmdline()

	if args.Listen != "" && args.Connect == "" && args.Socks5Proxy == "" && args.HttpProxy == "" && args.TorProxy == "" {
		args.Mode = NORMAL_PASSIVE
		printer.Warning("[*] Starting admin node on port %s\r\n", args.Listen)
	} else if args.Connect != "" && args.Listen == "" && args.Socks5Proxy == "" && args.HttpProxy == "" && args.TorProxy == "" {
		args.Mode = NORMAL_ACTIVE
		printer.Warning("[*] Trying to connect node actively")
	} else if args.Connect != "" && args.Listen == "" && args.Socks5Proxy != "" && args.HttpProxy == "" {
		args.Mode = SOCKS5_PROXY_ACTIVE
		printer.Warning("[*] Trying to connect node actively via socks5 proxy %s\r\n", args.Socks5Proxy)
	} else if args.Connect != "" && args.Listen == "" && args.Socks5Proxy == "" && args.HttpProxy != "" && args.TorProxy == "" {
		args.Mode = HTTP_PROXY_ACTIVE
		printer.Warning("[*] Trying to connect node actively via http proxy %s\r\n", args.HttpProxy)
	} else if args.Connect != "" && args.Listen == "" && args.Socks5Proxy == "" && args.HttpProxy == "" && args.TorProxy != "" {
		args.Mode = TOR_PROXY_ACTIVE
		printer.Warning("[*] Trying to connect node actively via Tor proxy %s\r\n", args.TorProxy)
	} else {
		flag.Usage()
		os.Exit(0)
	}

	if args.Domain == "" && args.Connect != "" {
		addrSlice := strings.SplitN(args.Connect, ":", 2)
		args.Domain = addrSlice[0]
	}

	if err := checkOptions(args); err != nil {
		ExitCleanup()
		printer.Fail("[*] Options err: %s\r\n", err.Error())
		os.Exit(0)
	}

	applyProtocolFingerprint(args)

	return args
}

func scrubSecretArgs() {
	for i := 1; i < len(os.Args); i++ {
		if os.Args[i] == "-s" && i+1 < len(os.Args) {
			os.Args[i+1] = strings.Repeat("x", len(os.Args[i+1]))
			i++
			continue
		}
		if strings.HasPrefix(os.Args[i], "-s=") {
			os.Args[i] = "-s=" + strings.Repeat("x", len(strings.TrimPrefix(os.Args[i], "-s=")))
		}
		if os.Args[i] == "--passphrase" && i+1 < len(os.Args) {
			os.Args[i+1] = strings.Repeat("x", len(os.Args[i+1]))
			i++
			continue
		}
		if strings.HasPrefix(os.Args[i], "--passphrase=") {
			os.Args[i] = "--passphrase=" + strings.Repeat("x", len(strings.TrimPrefix(os.Args[i], "--passphrase=")))
		}
	}
}

func applyProtocolFingerprint(option *Options) {
	if option.Magic != "" {
		if err := share.SetMagic([]byte(option.Magic)); err != nil {
			ExitCleanup()
			printer.Fail("[*] Options err: %s\r\n", err.Error())
			os.Exit(0)
		}
	}
	if option.WSPath != "" {
		if err := protocol.SetWebSocketPath(option.WSPath); err != nil {
			ExitCleanup()
			printer.Fail("[*] Options err: %s\r\n", err.Error())
			os.Exit(0)
		}
	}
}

func checkOptions(option *Options) error {
	if option.TlsEnable && option.TlsFingerprint == "" && !option.TlsInsecure {
		return errors.New("--tls-enable requires --tls-fingerprint or --tls-insecure")
	}

	var err error

	if args.Connect != "" && !share.IsOnionAddress(args.Connect) {
		_, err = net.ResolveTCPAddr("", option.Connect)
	}

	if args.Socks5Proxy != "" {
		_, err = net.ResolveTCPAddr("", option.Socks5Proxy)
	}

	if args.HttpProxy != "" {
		_, err = net.ResolveTCPAddr("", option.HttpProxy)
	}

	if args.TorProxy != "" {
		_, err = net.ResolveTCPAddr("", option.TorProxy)
	}

	return err
}
