package initial

import (
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	"Shroud/identity"
	"Shroud/protocol"
	"Shroud/share"
	"Shroud/share/transport"
	"Shroud/utils"

	reuseport "github.com/libp2p/go-reuseport"
)

var chainName = "CT"

var START_FORWARDING string
var STOP_FORWARDING string

func initChainName(secret []byte) {
	if len(secret) == 0 {
		chainName = "CT"
		return
	}
	h := sha256.Sum256(append(secret, []byte("iptables-chain")...))
	chainName = "CT" + hex.EncodeToString(h[:3])
}

func logVersionCheck(peerVersion string) {
	if peerVersion == "" {
		log.Printf("[*] Warning: peer is running an older version without version negotiation")
	} else if peerVersion != protocol.SHROUD_VERSION {
		log.Printf("[*] Warning: version mismatch: local=%s remote=%s", protocol.SHROUD_VERSION, peerVersion)
	}
}

func achieveUUID(conn net.Conn, cryptoKey []byte, linkKey []byte) (uuid string) {
	rMessage := protocol.NewUpMsg(conn, cryptoKey, linkKey, protocol.TEMP_UUID)
	fHeader, fMessage, err := protocol.DestructMessage(rMessage)

	if err != nil {
		conn.Close()
		log.Fatalf("[*] Fail to achieve UUID, Error: %s", err.Error())
	}

	if fHeader.MessageType == protocol.UUID {
		mmess := fMessage.(*protocol.UUIDMess)
		uuid = mmess.UUID
	}

	return uuid
}

func NormalActive(userOptions *Options, cryptoKey []byte, proxy share.Proxy, agentID *identity.AgentStore) (net.Conn, string, []byte) {
	var sMessage, rMessage protocol.Message
	// just say hi!
	hiMess := &protocol.HIMess{
		GreetingLen: uint16(len(share.GreetHello())),
		Greeting:    share.GreetHello(),
		UUIDLen:     uint16(len(protocol.TEMP_UUID)),
		UUID:        protocol.TEMP_UUID,
		IsAdmin:     0,
		IsReconnect: 0,
		VersionLen:  uint16(len(protocol.SHROUD_VERSION)),
		Version:     protocol.SHROUD_VERSION,
	}

	header := &protocol.Header{
		Sender:      protocol.TEMP_UUID,
		Accepter:    protocol.ADMIN_UUID,
		MessageType: protocol.HI,
		RouteLen:    uint32(len([]byte(protocol.TEMP_ROUTE))),
		Route:       protocol.TEMP_ROUTE,
	}

	for {
		var (
			conn net.Conn
			err  error
		)

		if proxy == nil {
			conn, err = net.Dial("tcp", userOptions.Connect)
		} else {
			conn, err = proxy.Dial()
		}

		if err != nil {
			log.Fatalf("[*] Error occurred: %s", err.Error())
		}
		utils.EnableKeepAlive(conn)

		if userOptions.TlsEnable {
			var tlsConfig *tls.Config
			tlsConfig, err = transport.NewClientTLSConfig(userOptions.Domain, userOptions.TlsFingerprint, userOptions.TlsInsecure)
			if err != nil {
				log.Printf("[*] Error occurred: %s", err.Error())
				conn.Close()
				continue
			}
			conn = transport.WrapTLSClientConn(conn, tlsConfig)
		}

		param := new(protocol.NegParam)
		param.Conn = conn
		param.Domain = userOptions.Domain
		proto := protocol.NewUpProto(param)
		proto.CNegotiate()

		var linkKey []byte
		linkKey, err = share.ActiveAgentAuthAndExchange(conn, agentID)
		if err != nil {
			log.Fatalf("[*] Error occurred: %s", err.Error())
		}

		sMessage = protocol.NewUpMsg(conn, cryptoKey, linkKey, protocol.TEMP_UUID)

		protocol.ConstructMessage(sMessage, header, hiMess, false)
		sMessage.SendMessage()

		rMessage = protocol.NewUpMsg(conn, cryptoKey, linkKey, protocol.TEMP_UUID)
		fHeader, fMessage, err := protocol.DestructMessage(rMessage)

		if err != nil {
			conn.Close()
			log.Fatalf("[*] Fail to connect %s, Error: %s", conn.RemoteAddr().String(), err.Error())
		}

		if fHeader.MessageType == protocol.HI {
			mmess := fMessage.(*protocol.HIMess)
			if mmess.Greeting == share.GreetAck() && mmess.IsAdmin == 1 {
				logVersionCheck(mmess.Version)
				uuid := achieveUUID(conn, cryptoKey, linkKey)
				return conn, uuid, linkKey
			}
		}

		conn.Close()
		log.Fatal("[*] Admin looks invalid!\n")
	}
}

func NormalPassive(userOptions *Options, cryptoKey []byte, agentID *identity.AgentStore) (net.Conn, string, []byte) {
	listenAddr, _, err := utils.CheckIPPort(userOptions.Listen)
	if err != nil {
		log.Fatalf("[*] Error occurred: %s", err.Error())
	}

	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Fatalf("[*] Error occurred: %s", err.Error())
	}

	defer func() {
		listener.Close()
	}()

	var sMessage, rMessage protocol.Message

	hiMess := &protocol.HIMess{
		GreetingLen: uint16(len(share.GreetAck())),
		Greeting:    share.GreetAck(),
		UUIDLen:     uint16(len(protocol.TEMP_UUID)),
		UUID:        protocol.TEMP_UUID,
		IsAdmin:     0,
		IsReconnect: 0,
		VersionLen:  uint16(len(protocol.SHROUD_VERSION)),
		Version:     protocol.SHROUD_VERSION,
	}

	header := &protocol.Header{
		Sender:      protocol.TEMP_UUID,
		Accepter:    protocol.ADMIN_UUID,
		MessageType: protocol.HI,
		RouteLen:    uint32(len([]byte(protocol.TEMP_ROUTE))),
		Route:       protocol.TEMP_ROUTE,
	}

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("[*] Error occurred: %s\n", err.Error())
			continue
		}
		utils.EnableKeepAlive(conn)

		if userOptions.TlsEnable {
			var tlsConfig *tls.Config
			tlsConfig, err = transport.NewServerTLSConfig()
			if err != nil {
				log.Printf("[*] Error occurred: %s", err.Error())
				conn.Close()
				continue
			}
			conn = transport.WrapTLSServerConn(conn, tlsConfig)
		}

		param := new(protocol.NegParam)
		param.Conn = conn
		proto := protocol.NewUpProto(param)
		proto.SNegotiate()

		var linkKey []byte
		linkKey, _, err = share.PassiveAgentAuthAndExchange(conn, agentID)
		if err != nil {
			log.Fatalf("[*] Error occurred: %s", err.Error())
		}

		rMessage = protocol.NewUpMsg(conn, cryptoKey, linkKey, protocol.TEMP_UUID)
		fHeader, fMessage, err := protocol.DestructMessage(rMessage)

		if err != nil {
			log.Printf("[*] Fail to set connection from %s, Error: %s\n", conn.RemoteAddr().String(), err.Error())
			conn.Close()
			continue
		}

		if fHeader.MessageType == protocol.HI {
			mmess := fMessage.(*protocol.HIMess)
			if mmess.Greeting == share.GreetHello() && mmess.IsAdmin == 1 {
				logVersionCheck(mmess.Version)
				sMessage = protocol.NewUpMsg(conn, cryptoKey, linkKey, protocol.TEMP_UUID)
				protocol.ConstructMessage(sMessage, header, hiMess, false)
				sMessage.SendMessage()
				uuid := achieveUUID(conn, cryptoKey, linkKey)
				return conn, uuid, linkKey
			}
		}

		conn.Close()
		log.Println("[*] Incoming connection looks invalid.")
	}
}

func TorHiddenPassive(userOptions *Options, cryptoKey []byte, agentID *identity.AgentStore) (net.Conn, string, []byte) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatalf("[*] Error occurred: %s", err.Error())
	}

	localPort := listener.Addr().(*net.TCPAddr).Port

	tc := share.NewTorControl(userOptions.TorControl, userOptions.TorControlPW)
	if err := tc.Connect(); err != nil {
		listener.Close()
		log.Fatalf("[*] Cannot connect to Tor control port: %s", err.Error())
	}
	if err := tc.Authenticate(); err != nil {
		listener.Close()
		tc.Close()
		log.Fatalf("[*] Tor control authentication failed: %s", err.Error())
	}

	onionAddr, err := tc.AddOnion(localPort, localPort)
	if err != nil {
		listener.Close()
		tc.Close()
		log.Fatalf("[*] Failed to create Tor hidden service: %s", err.Error())
	}

	log.Printf("[*] Tor hidden service started: %s:%d\n", onionAddr, localPort)

	defer func() {
		listener.Close()
	}()

	var sMessage, rMessage protocol.Message

	hiMess := &protocol.HIMess{
		GreetingLen: uint16(len(share.GreetAck())),
		Greeting:    share.GreetAck(),
		UUIDLen:     uint16(len(protocol.TEMP_UUID)),
		UUID:        protocol.TEMP_UUID,
		IsAdmin:     0,
		IsReconnect: 0,
		VersionLen:  uint16(len(protocol.SHROUD_VERSION)),
		Version:     protocol.SHROUD_VERSION,
	}

	header := &protocol.Header{
		Sender:      protocol.TEMP_UUID,
		Accepter:    protocol.ADMIN_UUID,
		MessageType: protocol.HI,
		RouteLen:    uint32(len([]byte(protocol.TEMP_ROUTE))),
		Route:       protocol.TEMP_ROUTE,
	}

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("[*] Error occurred: %s\n", err.Error())
			continue
		}
		utils.EnableKeepAlive(conn)

		param := new(protocol.NegParam)
		param.Conn = conn
		proto := protocol.NewUpProto(param)
		proto.SNegotiate()

		var linkKey []byte
		linkKey, _, err = share.PassiveAgentAuthAndExchange(conn, agentID)
		if err != nil {
			conn.Close()
			continue
		}

		rMessage = protocol.NewUpMsg(conn, cryptoKey, linkKey, protocol.TEMP_UUID)
		fHeader, fMessage, err := protocol.DestructMessage(rMessage)

		if err != nil {
			log.Printf("[*] Fail to set connection from %s, Error: %s\n", conn.RemoteAddr().String(), err.Error())
			conn.Close()
			continue
		}

		if fHeader.MessageType == protocol.HI {
			mmess := fMessage.(*protocol.HIMess)
			if mmess.Greeting == share.GreetHello() && mmess.IsAdmin == 1 {
				logVersionCheck(mmess.Version)
				sMessage = protocol.NewUpMsg(conn, cryptoKey, linkKey, protocol.TEMP_UUID)
				protocol.ConstructMessage(sMessage, header, hiMess, false)
				sMessage.SendMessage()
				uuid := achieveUUID(conn, cryptoKey, linkKey)
				return conn, uuid, linkKey
			}
		}

		conn.Close()
		log.Println("[*] Incoming connection looks invalid.")
	}
}

// IPTable reuse port functions
func IPTableReusePassive(userOptions *Options, cryptoKey []byte, agentID *identity.AgentStore) (net.Conn, string, []byte) {
	initChainName(userOptions.Secret)
	setReuseSecret(userOptions)
	SetPortReuseRules(userOptions.Listen, userOptions.ReusePort)
	go waitForExit(userOptions.Listen, userOptions.ReusePort)
	conn, uuid, linkKey := NormalPassive(userOptions, cryptoKey, agentID)
	return conn, uuid, linkKey
}

func waitForExit(localPort, reusedPort string) {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	for {
		<-sigs
		DeletePortReuseRules(localPort, reusedPort)
		os.Exit(0)
	}
}

func setReuseSecret(userOptions *Options) {
	firstSecret := utils.GetStringMd5(string(userOptions.Secret))
	secondSecret := utils.GetStringMd5(firstSecret)
	finalSecret := firstSecret[:24] + secondSecret[:24]
	START_FORWARDING = finalSecret[16:32]
	STOP_FORWARDING = finalSecret[32:]
}

func DeletePortReuseRules(localPort string, reusedPort string) error {
	var cmds []string

	cmds = append(cmds, fmt.Sprintf("iptables -t nat -D PREROUTING -p tcp --dport %s --syn -m recent --rcheck --seconds 3600 --name %s --rsource -j %s", reusedPort, strings.ToLower(chainName), chainName))
	cmds = append(cmds, fmt.Sprintf("iptables -D INPUT -p tcp -m string --string %s --algo bm -m recent --name %s --remove -j ACCEPT", STOP_FORWARDING, strings.ToLower(chainName)))
	cmds = append(cmds, fmt.Sprintf("iptables -D INPUT -p tcp -m string --string %s --algo bm -m recent --set --name %s --rsource -j ACCEPT", START_FORWARDING, strings.ToLower(chainName)))
	cmds = append(cmds, fmt.Sprintf("iptables -t nat -F %s", chainName))
	cmds = append(cmds, fmt.Sprintf("iptables -t nat -X %s", chainName))

	for _, each := range cmds {
		cmd := strings.Split(each, " ")
		exec.Command(cmd[0], cmd[1:]...).Run()
	}

	return nil
}

func SetPortReuseRules(localPort string, reusedPort string) error {
	var cmds []string

	cmds = append(cmds, fmt.Sprintf("iptables -t nat -N %s", chainName))                                                                                                                                      //新建自定义链
	cmds = append(cmds, fmt.Sprintf("iptables -t nat -A %s -p tcp -j REDIRECT --to-port %s", chainName, localPort))                                                                                           //将自定义链定义为转发流量至自定义监听端口
	cmds = append(cmds, fmt.Sprintf("iptables -A INPUT -p tcp -m string --string %s --algo bm -m recent --set --name %s --rsource -j ACCEPT", START_FORWARDING, strings.ToLower(chainName)))                  //设置当有一个报文带着特定字符串经过INPUT链时，将此报文的源地址加入一个特定列表中
	cmds = append(cmds, fmt.Sprintf("iptables -A INPUT -p tcp -m string --string %s --algo bm -m recent --name %s --remove -j ACCEPT", STOP_FORWARDING, strings.ToLower(chainName)))                          //设置当有一个报文带着特定字符串经过INPUT链时，将此报文的源地址从一个特定列表中移除
	cmds = append(cmds, fmt.Sprintf("iptables -t nat -A PREROUTING -p tcp --dport %s --syn -m recent --rcheck --seconds 3600 --name %s --rsource -j %s", reusedPort, strings.ToLower(chainName), chainName)) // 设置当有任意报文访问指定的复用端口时，检查特定列表，如果此报文的源地址在特定列表中且不超过3600秒，则执行自定义链

	for _, each := range cmds {
		cmd := strings.Split(each, " ")
		exec.Command(cmd[0], cmd[1:]...).Run() //添加规则
	}

	return nil
}

// soreuse port functions
func SoReusePassive(userOptions *Options, cryptoKey []byte, agentID *identity.AgentStore) (net.Conn, string, []byte) {
	listenAddr := fmt.Sprintf("%s:%s", userOptions.ReuseHost, userOptions.ReusePort)

	listener, err := reuseport.Listen("tcp", listenAddr)
	if err != nil {
		log.Fatalf("[*] Error occurred: %s", err.Error())
	}

	defer func() {
		listener.Close()
	}()

	var sMessage, rMessage protocol.Message

	hiMess := &protocol.HIMess{
		GreetingLen: uint16(len(share.GreetAck())),
		Greeting:    share.GreetAck(),
		UUIDLen:     uint16(len(protocol.TEMP_UUID)),
		UUID:        protocol.TEMP_UUID,
		IsAdmin:     0,
		IsReconnect: 0,
		VersionLen:  uint16(len(protocol.SHROUD_VERSION)),
		Version:     protocol.SHROUD_VERSION,
	}

	header := &protocol.Header{
		Sender:      protocol.TEMP_UUID,
		Accepter:    protocol.ADMIN_UUID,
		MessageType: protocol.HI,
		RouteLen:    uint32(len([]byte(protocol.TEMP_ROUTE))),
		Route:       protocol.TEMP_ROUTE,
	}

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("[*] Error occurred: %s\n", err.Error())
			continue
		}
		utils.EnableKeepAlive(conn)

		if userOptions.TlsEnable {
			var tlsConfig *tls.Config
			tlsConfig, err = transport.NewServerTLSConfig()
			if err != nil {
				log.Printf("[*] Error occurred: %s", err.Error())
				conn.Close()
				continue
			}
			conn = transport.WrapTLSServerConn(conn, tlsConfig)
		}

		param := new(protocol.NegParam)
		param.Conn = conn
		proto := protocol.NewUpProto(param)
		proto.SNegotiate()

		linkKey, _, err := share.SoReuseAgentAuthAndExchange(conn, userOptions.ReusePort, agentID)
		if err != nil {
			continue
		}

		rMessage = protocol.NewUpMsg(conn, cryptoKey, linkKey, protocol.TEMP_UUID)
		fHeader, fMessage, err := protocol.DestructMessage(rMessage)

		if err != nil {
			log.Printf("[*] Fail to set connection from %s, Error: %s\n", conn.RemoteAddr().String(), err.Error())
			conn.Close()
			continue
		}

		if fHeader.MessageType == protocol.HI {
			mmess := fMessage.(*protocol.HIMess)
			if mmess.Greeting == share.GreetHello() && mmess.IsAdmin == 1 {
				logVersionCheck(mmess.Version)
				sMessage = protocol.NewUpMsg(conn, cryptoKey, linkKey, protocol.TEMP_UUID)
				protocol.ConstructMessage(sMessage, header, hiMess, false)
				sMessage.SendMessage()
				uuid := achieveUUID(conn, cryptoKey, linkKey)
				return conn, uuid, linkKey
			}
		}

		conn.Close()
		log.Println("[*] Incoming connection looks invalid.")
	}
}

// conn is not for shroud, proxy conn to right port
func ProxyStream(conn net.Conn, message []byte, report string) {
	reuseAddr := fmt.Sprintf("127.0.0.1:%s", report)

	reuseConn, err := net.Dial("tcp", reuseAddr)

	if err != nil {
		fmt.Println(err)
		return
	}
	// send back the bytes we read before
	reuseConn.Write(message)

	go CopyTraffic(conn, reuseConn)
	CopyTraffic(reuseConn, conn)
}

func CopyTraffic(input, output net.Conn) {
	defer input.Close()

	buf := make([]byte, 10240)

	for {
		count, err := input.Read(buf)
		if err != nil {
			if err == io.EOF && count > 0 {
				output.Write(buf[:count])
			}
			break
		}
		if count > 0 {
			output.Write(buf[:count])
		}
	}
}
