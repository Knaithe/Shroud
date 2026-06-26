package protocol

import (
	"bytes"
	crand "crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"

	"Shroud/crypto"
	"Shroud/identity"
	"Shroud/utils"
)

const (
	MaxRouteLen = 4096
	MaxDataLen  = 32 << 20 // 32 MB
)

type RawProto struct{}

type RawMessage struct {
	// Essential component to apply a Message
	UUID            string
	Conn            net.Conn
	CryptoSecret    []byte
	LinkKey         []byte
	E2EKey          []byte
	E2EKeyResolver  func(string) []byte
	CommandSigner   *identity.AdminStore
	CommandVerifier *identity.AgentStore
	// Prepared buffer
	HeaderBuffer []byte
	DataBuffer   []byte
}

func (proto *RawProto) CNegotiate() error { return nil }

func (proto *RawProto) SNegotiate() error { return nil }

func (message *RawMessage) ConstructHeader() {}

func (message *RawMessage) ConstructData(header *Header, mess interface{}, isPass bool) {
	var headerBuffer, dataBuffer bytes.Buffer
	// First, construct own header
	messageTypeBuf := make([]byte, 2)
	routeLenBuf := make([]byte, 4)

	binary.BigEndian.PutUint16(messageTypeBuf, header.MessageType)
	binary.BigEndian.PutUint32(routeLenBuf, header.RouteLen)

	// Write header into buffer(except for dataLen)
	headerBuffer.Write([]byte(header.Sender))
	headerBuffer.Write([]byte(header.Accepter))
	headerBuffer.Write(messageTypeBuf)
	headerBuffer.Write(routeLenBuf)
	headerBuffer.Write([]byte(header.Route))

	if !isPass {
		dataBuffer.Write(mess.(Marshalable).MarshalBinary())
	} else {
		dataBuffer.Write(mess.([]byte))
	}

	message.DataBuffer = dataBuffer.Bytes()
	// Encrypt&Compress data. When a per-target E2E key is available, wrap the
	// plaintext command/result body before legacy payload encryption so that
	// intermediate agents can route but cannot read or alter executable content.
	if !isPass {
		e2eKey := message.e2eKeyForConstruct(header)
		var err error
		headerAAD := headerSigningAAD(header)
		if message.CommandSigner != nil && isAdminCommand(header.MessageType) {
			message.DataBuffer, err = message.CommandSigner.SignCommandPayload(headerAAD, message.DataBuffer)
			if err != nil {
				log.Printf("[*] command sign error, aborting send: %s", err.Error())
				message.DataBuffer = nil
				return
			}
		}
		if e2eKey != nil {
			message.DataBuffer, err = crypto.AESEncrypt(message.DataBuffer, e2eKey)
			if err != nil {
				log.Printf("[*] e2e encrypt error, aborting send: %s", err.Error())
				message.DataBuffer = nil
				return
			}
		}
		compressed, gzErr := crypto.GzipCompressE(message.DataBuffer)
		if gzErr != nil {
			log.Printf("[*] gzip compress error, aborting send: %s", gzErr.Error())
			message.DataBuffer = nil
			return
		}
		message.DataBuffer = compressed
		if message.CryptoSecret != nil {
			encrypted, err := crypto.AESEncrypt(message.DataBuffer, message.CryptoSecret)
			if err != nil {
				log.Printf("[*] payload encrypt error, aborting send: %s", err.Error())
				message.DataBuffer = nil
				return
			}
			message.DataBuffer = encrypted
		}
	}
	// Calculate the whole data's length
	dataLenBuf := make([]byte, 8)
	binary.BigEndian.PutUint64(dataLenBuf, uint64(len(message.DataBuffer)))
	headerBuffer.Write(dataLenBuf)
	message.HeaderBuffer = headerBuffer.Bytes()
}

func (message *RawMessage) ConstructSuffix() {}

func (message *RawMessage) DeconstructHeader() {}

const MaxFrameLen = MaxDataLen + MaxRouteLen + 128

func (message *RawMessage) DeconstructData() (*Header, interface{}, error) {
	var plainFrame []byte

	if message.LinkKey != nil {
		frameLenBuf := make([]byte, 8)
		if _, err := io.ReadFull(message.Conn, frameLenBuf); err != nil {
			return nil, nil, err
		}
		frameLen := binary.BigEndian.Uint64(frameLenBuf)
		if frameLen > MaxFrameLen {
			return nil, nil, fmt.Errorf("frame length %d exceeds maximum %d", frameLen, MaxFrameLen)
		}
		encFrame := make([]byte, frameLen)
		if _, err := io.ReadFull(message.Conn, encFrame); err != nil {
			return nil, nil, err
		}
		var err error
		plainFrame, err = crypto.AESDecrypt(encFrame, message.LinkKey)
		if err != nil {
			return nil, nil, fmt.Errorf("link decrypt: %w", err)
		}
		return message.parseFrame(plainFrame)
	}

	return message.parseFrameFromConn()
}

func (message *RawMessage) parseFrame(frame []byte) (*Header, interface{}, error) {
	r := bytes.NewReader(frame)
	header := new(Header)

	senderBuf := make([]byte, 10)
	if _, err := io.ReadFull(r, senderBuf); err != nil {
		return nil, nil, err
	}
	header.Sender = string(senderBuf)

	accepterBuf := make([]byte, 10)
	if _, err := io.ReadFull(r, accepterBuf); err != nil {
		return nil, nil, err
	}
	header.Accepter = string(accepterBuf)

	messageTypeBuf := make([]byte, 2)
	if _, err := io.ReadFull(r, messageTypeBuf); err != nil {
		return nil, nil, err
	}
	header.MessageType = binary.BigEndian.Uint16(messageTypeBuf)

	routeLenBuf := make([]byte, 4)
	if _, err := io.ReadFull(r, routeLenBuf); err != nil {
		return nil, nil, err
	}
	header.RouteLen = binary.BigEndian.Uint32(routeLenBuf)
	if header.RouteLen > MaxRouteLen {
		return nil, nil, fmt.Errorf("route length %d exceeds maximum %d", header.RouteLen, MaxRouteLen)
	}

	routeBuf := make([]byte, header.RouteLen)
	if _, err := io.ReadFull(r, routeBuf); err != nil {
		return nil, nil, err
	}
	header.Route = string(routeBuf)

	dataLenBuf := make([]byte, 8)
	if _, err := io.ReadFull(r, dataLenBuf); err != nil {
		return nil, nil, err
	}
	header.DataLen = binary.BigEndian.Uint64(dataLenBuf)
	if header.DataLen > MaxDataLen {
		return nil, nil, fmt.Errorf("data length %d exceeds maximum %d", header.DataLen, MaxDataLen)
	}

	dataBuf := make([]byte, header.DataLen)
	if _, err := io.ReadFull(r, dataBuf); err != nil {
		return nil, nil, err
	}

	return message.processPayload(header, dataBuf)
}

func (message *RawMessage) parseFrameFromConn() (*Header, interface{}, error) {
	header := new(Header)

	senderBuf := make([]byte, 10)
	if _, err := io.ReadFull(message.Conn, senderBuf); err != nil {
		return nil, nil, err
	}
	header.Sender = string(senderBuf)

	accepterBuf := make([]byte, 10)
	if _, err := io.ReadFull(message.Conn, accepterBuf); err != nil {
		return nil, nil, err
	}
	header.Accepter = string(accepterBuf)

	messageTypeBuf := make([]byte, 2)
	if _, err := io.ReadFull(message.Conn, messageTypeBuf); err != nil {
		return nil, nil, err
	}
	header.MessageType = binary.BigEndian.Uint16(messageTypeBuf)

	routeLenBuf := make([]byte, 4)
	if _, err := io.ReadFull(message.Conn, routeLenBuf); err != nil {
		return nil, nil, err
	}
	header.RouteLen = binary.BigEndian.Uint32(routeLenBuf)
	if header.RouteLen > MaxRouteLen {
		return nil, nil, fmt.Errorf("route length %d exceeds maximum %d", header.RouteLen, MaxRouteLen)
	}

	routeBuf := make([]byte, header.RouteLen)
	if _, err := io.ReadFull(message.Conn, routeBuf); err != nil {
		return nil, nil, err
	}
	header.Route = string(routeBuf)

	dataLenBuf := make([]byte, 8)
	if _, err := io.ReadFull(message.Conn, dataLenBuf); err != nil {
		return nil, nil, err
	}
	header.DataLen = binary.BigEndian.Uint64(dataLenBuf)
	if header.DataLen > MaxDataLen {
		return nil, nil, fmt.Errorf("data length %d exceeds maximum %d", header.DataLen, MaxDataLen)
	}

	dataBuf := make([]byte, header.DataLen)
	if _, err := io.ReadFull(message.Conn, dataBuf); err != nil {
		return nil, nil, err
	}

	return message.processPayload(header, dataBuf)
}

func (message *RawMessage) processPayload(header *Header, dataBuf []byte) (*Header, interface{}, error) {
	if header.Accepter == TEMP_UUID || message.UUID == ADMIN_UUID || message.UUID == header.Accepter {
		if message.CryptoSecret != nil {
			decrypted, err := crypto.AESDecrypt(dataBuf, message.CryptoSecret)
			if err != nil {
				return header, nil, err
			}
			dataBuf = decrypted
		}
	} else {
		return header, dataBuf, nil
	}

	dataBuf, err := crypto.GzipDecompress(dataBuf)
	if err != nil {
		return header, nil, fmt.Errorf("decompression failed: %w", err)
	}
	e2eKey := message.e2eKeyForProcess(header)
	if e2eKey != nil {
		dataBuf, err = crypto.AESDecrypt(dataBuf, e2eKey)
		if err != nil {
			return header, nil, fmt.Errorf("e2e decrypt failed: %w", err)
		}
	}
	if message.CommandVerifier != nil && isAdminCommand(header.MessageType) && header.Sender == ADMIN_UUID && message.UUID == header.Accepter {
		dataBuf, err = message.CommandVerifier.VerifyCommandPayload(headerSigningAAD(header), dataBuf)
		if err != nil {
			return header, nil, fmt.Errorf("command signature failed: %w", err)
		}
	}

	var mess interface{}
	switch header.MessageType {
	case HI:
		mess = new(HIMess)
	case UUID:
		mess = new(UUIDMess)
	case CHILDUUIDREQ:
		mess = new(ChildUUIDReq)
	case CHILDUUIDRES:
		mess = new(ChildUUIDRes)
	case MYINFO:
		mess = new(MyInfo)
	case MYMEMO:
		mess = new(MyMemo)
	case SHELLREQ:
		mess = new(ShellReq)
	case SHELLRES:
		mess = new(ShellRes)
	case SHELLCOMMAND:
		mess = new(ShellCommand)
	case SHELLRESULT:
		mess = new(ShellResult)
	case SHELLEXIT:
		mess = new(ShellExit)
	case LISTENREQ:
		mess = new(ListenReq)
	case LISTENRES:
		mess = new(ListenRes)
	case SSHREQ:
		mess = new(SSHReq)
	case SSHRES:
		mess = new(SSHRes)
	case SSHCOMMAND:
		mess = new(SSHCommand)
	case SSHRESULT:
		mess = new(SSHResult)
	case SSHEXIT:
		mess = new(SSHExit)
	case SSHTUNNELREQ:
		mess = new(SSHTunnelReq)
	case SSHTUNNELRES:
		mess = new(SSHTunnelRes)
	case FILESTATREQ:
		mess = new(FileStatReq)
	case FILESTATRES:
		mess = new(FileStatRes)
	case FILEDATA:
		mess = new(FileData)
	case FILEERR:
		mess = new(FileErr)
	case FILEDOWNREQ:
		mess = new(FileDownReq)
	case FILEDOWNRES:
		mess = new(FileDownRes)
	case SOCKSSTART:
		mess = new(SocksStart)
	case SOCKSTCPDATA:
		mess = new(SocksTCPData)
	case SOCKSUDPDATA:
		mess = new(SocksUDPData)
	case UDPASSSTART:
		mess = new(UDPAssStart)
	case UDPASSRES:
		mess = new(UDPAssRes)
	case SOCKSTCPFIN:
		mess = new(SocksTCPFin)
	case SOCKSREADY:
		mess = new(SocksReady)
	case FORWARDTEST:
		mess = new(ForwardTest)
	case FORWARDSTART:
		mess = new(ForwardStart)
	case FORWARDREADY:
		mess = new(ForwardReady)
	case FORWARDDATA:
		mess = new(ForwardData)
	case FORWARDFIN:
		mess = new(ForwardFin)
	case BACKWARDTEST:
		mess = new(BackwardTest)
	case BACKWARDREADY:
		mess = new(BackwardReady)
	case BACKWARDSTART:
		mess = new(BackwardStart)
	case BACKWARDSEQ:
		mess = new(BackwardSeq)
	case BACKWARDDATA:
		mess = new(BackwardData)
	case BACKWARDFIN:
		mess = new(BackWardFin)
	case BACKWARDSTOP:
		mess = new(BackwardStop)
	case BACKWARDSTOPDONE:
		mess = new(BackwardStopDone)
	case CONNECTSTART:
		mess = new(ConnectStart)
	case CONNECTDONE:
		mess = new(ConnectDone)
	case NODEOFFLINE:
		mess = new(NodeOffline)
	case NODEREONLINE:
		mess = new(NodeReonline)
	case UPSTREAMOFFLINE:
		mess = new(UpstreamOffline)
	case UPSTREAMREONLINE:
		mess = new(UpstreamReonline)
	case SHUTDOWN:
		mess = new(Shutdown)
	case HEARTBEAT:
		mess = new(HeartbeatMsg)
	case HEARTBEATACK:
		mess = new(HeartbeatAckMsg)
	case TRANSPORTSWITCHREQ:
		mess = new(TransportSwitchReq)
	case TRANSPORTSWITCHRES:
		mess = new(TransportSwitchRes)
	case TRANSPORTSWITCHDONE:
		mess = new(TransportSwitchDone)
	case ROUTETABLE:
		mess = new(RouteTableMsg)
	case RSHELLLISTEN:
		mess = new(RShellListen)
	case RSHELLREADY:
		mess = new(RShellReady)
	case RSHELLCONN:
		mess = new(RShellConn)
	case RSHELLDATA:
		mess = new(RShellData)
	case RSHELLFIN:
		mess = new(RShellFin)
	case RSHELLSTOP:
		mess = new(RShellStop)
	case RSHELLSTOPDONE:
		mess = new(RShellStopDone)
	}

	if mess == nil {
		return header, nil, fmt.Errorf("unknown message type: %d", header.MessageType)
	}

	if err := mess.(Marshalable).UnmarshalBinary(dataBuf); err != nil {
		return header, nil, fmt.Errorf("unmarshal type %d: %w", header.MessageType, err)
	}

	return header, mess, nil
}

func (message *RawMessage) e2eKeyForConstruct(header *Header) []byte {
	if message.E2EKey != nil {
		return message.E2EKey
	}
	if !isE2EEligible(header.MessageType) {
		return nil
	}
	if message.E2EKeyResolver == nil || header == nil {
		return nil
	}
	// Admin -> enrolled target. Skip link-local TEMP_UUID messages and
	// parent-agent helper connections whose local UUID is not ADMIN_UUID.
	if message.UUID == ADMIN_UUID && header.Sender == ADMIN_UUID && header.Accepter != TEMP_UUID && header.Accepter != ADMIN_UUID {
		return message.E2EKeyResolver(header.Accepter)
	}
	// Agent -> real admin. TEMP_UUID first-contact messages must stay link-local
	// until the runtime UUID is assigned and identity context is installed.
	if header.Accepter == ADMIN_UUID && header.Sender != TEMP_UUID {
		return message.E2EKeyResolver(ADMIN_UUID)
	}
	return nil
}

func (message *RawMessage) e2eKeyForProcess(header *Header) []byte {
	if message.E2EKey != nil {
		return message.E2EKey
	}
	if !isE2EEligible(header.MessageType) {
		return nil
	}
	if message.E2EKeyResolver == nil || header == nil {
		return nil
	}
	// Target agent decrypting admin-originated command/data.
	if header.Sender == ADMIN_UUID && message.UUID == header.Accepter && header.Accepter != TEMP_UUID {
		return message.E2EKeyResolver(ADMIN_UUID)
	}
	// Real admin decrypting an enrolled agent's response/control message.
	if message.UUID == ADMIN_UUID && header.Accepter == ADMIN_UUID && header.Sender != TEMP_UUID {
		return message.E2EKeyResolver(header.Sender)
	}
	return nil
}

func isE2EEligible(t uint16) bool {
	switch t {
	case HI, UUID, CHILDUUIDREQ, CHILDUUIDRES:
		return false
	default:
		return true
	}
}

func (message *RawMessage) DeconstructSuffix() {}

var padSize int

func SetPadSize(n int) error {
	if n < 0 || n > 65536 {
		return fmt.Errorf("pad size must be 0..65536, got %d", n)
	}
	padSize = n
	return nil
}

func (message *RawMessage) SendMessage() {
	plainFrame := append(message.HeaderBuffer, message.DataBuffer...)

	if padSize > 0 && message.LinkKey != nil {
		frameLen := len(plainFrame)
		paddedLen := ((frameLen + padSize - 1) / padSize) * padSize
		if paddedLen > frameLen {
			padding := make([]byte, paddedLen-frameLen)
			crand.Read(padding)
			plainFrame = append(plainFrame, padding...)
		}
	}

	if message.LinkKey != nil {
		encFrame, err := crypto.AESEncrypt(plainFrame, message.LinkKey)
		if err != nil {
			log.Printf("[*] SendMessage link encrypt error: %s\n", err.Error())
			message.Conn.Close()
			message.HeaderBuffer = nil
			message.DataBuffer = nil
			return
		}
		frameLenBuf := make([]byte, 8)
		binary.BigEndian.PutUint64(frameLenBuf, uint64(len(encFrame)))
		if err := utils.WriteFull(message.Conn, append(frameLenBuf, encFrame...)); err != nil {
			log.Printf("[*] SendMessage write error: %s\n", err.Error())
		}
	} else {
		if err := utils.WriteFull(message.Conn, plainFrame); err != nil {
			log.Printf("[*] SendMessage write error: %s\n", err.Error())
		}
	}

	message.HeaderBuffer = nil
	message.DataBuffer = nil
}

func isAdminCommand(t uint16) bool {
	switch t {
	case SHELLREQ, SHELLCOMMAND, SHELLEXIT, LISTENREQ, SSHREQ, SSHCOMMAND, SSHEXIT, SSHTUNNELREQ, FILESTATREQ, FILEDATA, FILEERR, FILEDOWNREQ, SOCKSSTART, SOCKSTCPDATA, SOCKSUDPDATA, SOCKSTCPFIN, FORWARDTEST, FORWARDSTART, FORWARDDATA, FORWARDFIN, BACKWARDTEST, BACKWARDSTART, BACKWARDSEQ, BACKWARDDATA, BACKWARDFIN, BACKWARDSTOP, CONNECTSTART, SHUTDOWN, HEARTBEAT, TRANSPORTSWITCHREQ, TRANSPORTSWITCHDONE, MYMEMO, ROUTETABLE, RSHELLLISTEN, RSHELLDATA, RSHELLFIN, RSHELLSTOP:
		return true
	default:
		return false
	}
}

func headerSigningAAD(header *Header) []byte {
	var b bytes.Buffer
	b.WriteString(header.Sender)
	b.WriteString(header.Accepter)
	var mt [2]byte
	binary.BigEndian.PutUint16(mt[:], header.MessageType)
	b.Write(mt[:])
	return b.Bytes()
}
