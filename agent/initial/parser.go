package initial

import (
	"errors"
	"flag"
	"log"
	"net"
	"os"
	"strings"

	"Shroud/protocol"
	"Shroud/share"
	"Shroud/utils"
)

var secretFlag string

const (
	NORMAL_ACTIVE = iota
	NORMAL_RECONNECT_ACTIVE
	NORMAL_PASSIVE
	SOCKS5_PROXY_ACTIVE
	HTTP_PROXY_ACTIVE
	SOCKS5_PROXY_RECONNECT_ACTIVE
	HTTP_PROXY_RECONNECT_ACTIVE
	SO_REUSE_PASSIVE
	IPTABLES_REUSE_PASSIVE
	TOR_PROXY_ACTIVE
	TOR_PROXY_RECONNECT_ACTIVE
	TOR_HIDDEN_PASSIVE
)

type Options struct {
	Mode           int
	Secret         []byte
	Listen         string
	Reconnect      uint64
	Connect        string
	ReuseHost      string
	ReusePort      string
	Socks5Proxy    string
	Socks5ProxyU   string
	Socks5ProxyP   string
	HttpProxy      string
	TorProxy       string
	TorHidden      bool
	TorControl     string
	TorControlPW   string
	Upstream       string
	Downstream     string
	Charset        string
	Domain         string
	TlsEnable      bool
	TlsFingerprint string
	TlsInsecure    bool
	Magic          string
	WSPath         string
	Verbose        bool
	IdentityDir    string
	Passphrase     string
	IdentityPlain  bool
	PadSize        int
	UserAgent      string
	FrontDomain    string
	Origin         string
	KillDate       string
	WorkHours      string
	SelfDelete     bool
	SleepMask      bool
	Fileless       bool
}

var args *Options

func init() {
	args = new(Options)

	flag.StringVar(&secretFlag, "s", "", "")
	flag.StringVar(&args.Listen, "l", "", "")
	flag.Uint64Var(&args.Reconnect, "reconnect", 0, "")
	flag.StringVar(&args.Connect, "c", "", "")
	flag.StringVar(&args.ReuseHost, "rehost", "", "")
	flag.StringVar(&args.ReusePort, "report", "", "")
	flag.StringVar(&args.Socks5Proxy, "socks5-proxy", "", "")
	flag.StringVar(&args.Socks5ProxyU, "socks5-proxyu", "", "")
	flag.StringVar(&args.Socks5ProxyP, "socks5-proxyp", "", "")
	flag.StringVar(&args.HttpProxy, "http-proxy", "", "")
	flag.StringVar(&args.TorProxy, "tor-proxy", "", "")
	flag.BoolVar(&args.TorHidden, "tor-hidden", false, "")
	flag.StringVar(&args.TorControl, "tor-control", "127.0.0.1:9051", "")
	flag.StringVar(&args.TorControlPW, "tor-control-password", "", "")
	flag.StringVar(&args.Upstream, "up", "raw", "")
	flag.StringVar(&args.Downstream, "down", "raw", "")
	flag.StringVar(&args.Charset, "cs", "utf-8", "")
	flag.StringVar(&args.Domain, "domain", "", "")
	flag.BoolVar(&args.TlsEnable, "tls-enable", false, "")
	flag.StringVar(&args.TlsFingerprint, "tls-fingerprint", "", "")
	flag.BoolVar(&args.TlsInsecure, "tls-insecure", false, "")
	flag.StringVar(&args.Magic, "magic", "", "")
	flag.StringVar(&args.WSPath, "ws-path", "", "")
	flag.BoolVar(&args.Verbose, "v", false, "")
	flag.StringVar(&args.IdentityDir, "identity-dir", "", "")
	flag.StringVar(&args.Passphrase, "passphrase", "", "")
	flag.BoolVar(&args.IdentityPlain, "identity-plain", false, "")
	flag.IntVar(&args.PadSize, "pad-size", 0, "")
	flag.StringVar(&args.UserAgent, "user-agent", "", "")
	flag.StringVar(&args.FrontDomain, "front-domain", "", "")
	flag.StringVar(&args.Origin, "origin", "", "")
	flag.StringVar(&args.KillDate, "kill-date", "", "")
	flag.StringVar(&args.WorkHours, "work-hours", "", "")
	flag.BoolVar(&args.SelfDelete, "self-delete", false, "")
	flag.BoolVar(&args.SleepMask, "sleep-mask", false, "")
	flag.BoolVar(&args.Fileless, "fileless", false, "")

	flag.Usage = func() {}
}

// ParseOptions Parsing user's options
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

func ParseOptions() *Options {

	flag.Parse()
	args.Secret = resolveSecret()
	scrubSecretArgs()
	utils.ScrubCmdline()

	// Resolve sensitive credentials from environment variables
	if args.Socks5ProxyP == "" {
		if p := os.Getenv("SHROUD_SOCKS5_PROXY_PASS"); p != "" {
			args.Socks5ProxyP = p
			os.Unsetenv("SHROUD_SOCKS5_PROXY_PASS")
		}
	}
	if args.TorControlPW == "" {
		if p := os.Getenv("SHROUD_TOR_CONTROL_PASS"); p != "" {
			args.TorControlPW = p
			os.Unsetenv("SHROUD_TOR_CONTROL_PASS")
		}
	}

	if args.Listen != "" && args.Connect == "" && args.Reconnect == 0 && args.ReuseHost == "" && args.ReusePort == "" && args.Socks5Proxy == "" && args.HttpProxy == "" && args.TorProxy == "" && !args.TorHidden {
		args.Mode = NORMAL_PASSIVE
		log.Printf("[*] Starting agent node passively.Now listening on port %s\n", args.Listen)
	} else if args.Listen == "" && args.Connect != "" && args.Reconnect == 0 && args.ReuseHost == "" && args.ReusePort == "" && args.Socks5Proxy == "" && args.HttpProxy == "" && args.TorProxy == "" {
		args.Mode = NORMAL_ACTIVE
		log.Printf("[*] Starting agent node actively.Connecting to %s\n", args.Connect)
	} else if args.Listen == "" && args.Connect != "" && args.Reconnect != 0 && args.ReuseHost == "" && args.ReusePort == "" && args.Socks5Proxy == "" && args.HttpProxy == "" && args.TorProxy == "" {
		args.Mode = NORMAL_RECONNECT_ACTIVE
		log.Printf("[*] Starting agent node actively.Connecting to %s.Reconnecting every %d seconds\n", args.Connect, args.Reconnect)
	} else if args.Listen == "" && args.Connect == "" && args.Reconnect == 0 && args.ReuseHost != "" && args.ReusePort != "" && args.Socks5Proxy == "" && args.HttpProxy == "" && args.TorProxy == "" {
		args.Mode = SO_REUSE_PASSIVE
		log.Printf("[*] Starting agent node passively.Now reusing host %s, port %s(SO_REUSEPORT,SO_REUSEADDR)\n", args.ReuseHost, args.ReusePort)
	} else if args.Listen != "" && args.Connect == "" && args.Reconnect == 0 && args.ReuseHost == "" && args.ReusePort != "" && args.Socks5Proxy == "" && args.HttpProxy == "" && args.TorProxy == "" {
		args.Mode = IPTABLES_REUSE_PASSIVE
		log.Printf("[*] Starting agent node passively.Now reusing port %s(IPTABLES)\n", args.ReusePort)
	} else if args.Listen == "" && args.Connect != "" && args.Reconnect == 0 && args.ReuseHost == "" && args.ReusePort == "" && args.Socks5Proxy != "" && args.HttpProxy == "" { // ./shroud_agent -c <ip:port> -s [secret] --proxy <ip:port> --proxyu [username] --proxyp [password]
		args.Mode = SOCKS5_PROXY_ACTIVE
		log.Printf("[*] Starting agent node actively.Connecting to %s via socks5 proxy %s\n", args.Connect, args.Socks5Proxy)
	} else if args.Listen == "" && args.Connect != "" && args.Reconnect != 0 && args.ReuseHost == "" && args.ReusePort == "" && args.Socks5Proxy != "" && args.HttpProxy == "" { // ./shroud_agent -c <ip:port> -s [secret] --proxy <ip:port> --proxyu [username] --proxyp [password] --reconnect <seconds>
		args.Mode = SOCKS5_PROXY_RECONNECT_ACTIVE
		log.Printf("[*] Starting agent node actively.Connecting to %s via socks5 proxy %s.Reconnecting every %d seconds\n", args.Connect, args.Socks5Proxy, args.Reconnect)
	} else if args.Listen == "" && args.Connect != "" && args.Reconnect == 0 && args.ReuseHost == "" && args.ReusePort == "" && args.Socks5Proxy == "" && args.HttpProxy != "" {
		args.Mode = HTTP_PROXY_ACTIVE
		log.Printf("[*] Starting agent node actively.Connecting to %s via http proxy %s\n", args.Connect, args.HttpProxy)
	} else if args.Listen == "" && args.Connect != "" && args.Reconnect != 0 && args.ReuseHost == "" && args.ReusePort == "" && args.Socks5Proxy == "" && args.HttpProxy != "" {
		args.Mode = HTTP_PROXY_RECONNECT_ACTIVE
		log.Printf("[*] Starting agent node actively.Connecting to %s via http proxy %s.Reconnecting every %d seconds\n", args.Connect, args.HttpProxy, args.Reconnect)
	} else if args.Listen == "" && args.Connect != "" && args.Reconnect == 0 && args.TorProxy != "" && args.Socks5Proxy == "" && args.HttpProxy == "" {
		args.Mode = TOR_PROXY_ACTIVE
		log.Printf("[*] Starting agent node actively.Connecting to %s via Tor proxy %s\n", args.Connect, args.TorProxy)
	} else if args.Listen == "" && args.Connect != "" && args.Reconnect != 0 && args.TorProxy != "" && args.Socks5Proxy == "" && args.HttpProxy == "" {
		args.Mode = TOR_PROXY_RECONNECT_ACTIVE
		log.Printf("[*] Starting agent node actively.Connecting to %s via Tor proxy %s.Reconnecting every %d seconds\n", args.Connect, args.TorProxy, args.Reconnect)
	} else if args.TorHidden && args.Connect == "" && args.Listen == "" {
		args.Mode = TOR_HIDDEN_PASSIVE
		log.Printf("[*] Starting agent node as Tor hidden service via control port %s\n", args.TorControl)
	} else {
		os.Exit(1)
	}

	if args.Charset != "utf-8" && args.Charset != "gbk" {
		log.Fatalf("[*] Charset must be set as 'utf-8'(default) or 'gbk'")
	}

	if args.Domain == "" && args.Connect != "" {
		addrSlice := strings.SplitN(args.Connect, ":", 2)
		args.Domain = addrSlice[0]
	}

	if err := checkOptions(args); err != nil {
		log.Fatalf("[*] Options err: %s\n", err.Error())
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
		if os.Args[i] == "--socks5-proxyp" && i+1 < len(os.Args) {
			os.Args[i+1] = strings.Repeat("x", len(os.Args[i+1]))
			i++
			continue
		}
		if strings.HasPrefix(os.Args[i], "--socks5-proxyp=") {
			os.Args[i] = "--socks5-proxyp=" + strings.Repeat("x", len(strings.TrimPrefix(os.Args[i], "--socks5-proxyp=")))
		}
		if os.Args[i] == "--tor-control-password" && i+1 < len(os.Args) {
			os.Args[i+1] = strings.Repeat("x", len(os.Args[i+1]))
			i++
			continue
		}
		if strings.HasPrefix(os.Args[i], "--tor-control-password=") {
			os.Args[i] = "--tor-control-password=" + strings.Repeat("x", len(strings.TrimPrefix(os.Args[i], "--tor-control-password=")))
		}
	}
}

func applyProtocolFingerprint(option *Options) {
	if option.Magic != "" {
		if err := share.SetMagic([]byte(option.Magic)); err != nil {
			log.Fatalf("[*] Options err: %s\n", err.Error())
		}
	}
	if option.WSPath != "" {
		if err := protocol.SetWebSocketPath(option.WSPath); err != nil {
			log.Fatalf("[*] Options err: %s\n", err.Error())
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

	if args.ReuseHost != "" {
		if addr := net.ParseIP(args.ReuseHost); addr == nil {
			err = errors.New("ReuseHost is not a valid IP addr")
		}
	}

	return err
}
