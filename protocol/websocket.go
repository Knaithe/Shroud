package protocol

import (
	"bytes"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"regexp"
	"strings"
	"time"

	"Shroud/utils"
)

// KNOWN ISSUE: WebSocket framing is not fully RFC6455 compliant.
// Missing: client-to-server masking, proper opcode handling, ping/pong, close frames.
// Works with Nginx reverse proxy in practice, but may fail with strict WAF/DPI.
// Full RFC6455 compliance or migration to gorilla/websocket is a future milestone.
const websocketGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

var websocketPath = mustRandomWebSocketPath()

func SetWebSocketPath(path string) error {
	if path == "" || path[0] != '/' || strings.ContainsAny(path, " \r\n\t") {
		return errors.New("websocket path must start with '/' and contain no whitespace")
	}
	websocketPath = path
	return nil
}

func WebSocketPath() string { return websocketPath }

func mustRandomWebSocketPath() string {
	b := make([]byte, 8)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "/ws"
	}
	return "/" + base64.RawURLEncoding.EncodeToString(b)
}

type WSProto struct {
	domain string
	conn   net.Conn
	*RawProto
}

func (proto *WSProto) CNegotiate() error {
	defer proto.conn.SetReadDeadline(time.Time{})
	proto.conn.SetReadDeadline(time.Now().Add(10 * time.Second))

	nonce, err := generateNonce()
	if err != nil {
		return err
	}
	expectedAccept, err := getNonceAccept(nonce)
	if err != nil {
		return err
	}

	host := EffectiveHost(proto.domain)
	origin := EffectiveOrigin()
	ua := RotateUserAgent()

	var sb strings.Builder
	fmt.Fprintf(&sb, "GET %s HTTP/1.1\r\n", websocketPath)
	fmt.Fprintf(&sb, "Host: %s\r\n", host)
	sb.WriteString("Upgrade: websocket\r\n")
	sb.WriteString("Connection: Upgrade\r\n")
	fmt.Fprintf(&sb, "Sec-WebSocket-Key: %s\r\n", string(nonce))
	fmt.Fprintf(&sb, "Origin: %s\r\n", origin)
	sb.WriteString("Sec-WebSocket-Version: 13\r\n")
	if ua != "" {
		fmt.Fprintf(&sb, "User-Agent: %s\r\n", ua)
	}
	sb.WriteString("\r\n")
	utils.WriteFull(proto.conn, []byte(sb.String()))

	result := bytes.Buffer{}
	buf := make([]byte, 1024)

	for {
		count, err := proto.conn.Read(buf)
		if err != nil {
			if err == io.EOF && count > 0 {
				result.Write(buf[:count])
			} else if timeoutErr, ok := err.(net.Error); ok && timeoutErr.Timeout() {
				return err
			}
			break
		}

		if count > 0 {
			result.Write(buf[:count])
			if result.Len() > 8192 {
				return errors.New("websocket header too large")
			}
			if bytes.HasSuffix(result.Bytes(), []byte("\r\n\r\n")) {
				break
			}
		}
	}

	resp := result.String()
	respLower := strings.ToLower(resp)
	if !strings.Contains(respLower, "upgrade: websocket") ||
		!containsHeaderValue(resp, "Sec-WebSocket-Accept", string(expectedAccept)) {
		return errors.New("not websocket protocol")
	}

	return nil
}

func (proto *WSProto) SNegotiate() error {
	defer proto.conn.SetReadDeadline(time.Time{})
	proto.conn.SetReadDeadline(time.Now().Add(10 * time.Second))

	result := bytes.Buffer{}
	buf := make([]byte, 1024)

	for {
		count, err := proto.conn.Read(buf)
		if err != nil {
			if err == io.EOF && count > 0 {
				result.Write(buf[:count])
			} else if timeoutErr, ok := err.(net.Error); ok && timeoutErr.Timeout() {
				return err
			}
			break
		}

		if count > 0 {
			result.Write(buf[:count])
			if result.Len() > 8192 {
				return errors.New("websocket header too large")
			}
			if bytes.HasSuffix(result.Bytes(), []byte("\r\n\r\n")) {
				break
			}
		}
	}

	re := regexp.MustCompile(`(?i)Sec-WebSocket-Key:\s*([A-Za-z0-9+/]{22}==)`)
	tkey := re.FindStringSubmatch(result.String())
	if len(tkey) < 2 {
		return errors.New("Sec-WebSocket-Key is not in header")
	}

	key := tkey[1]
	expectedAccept, err := getNonceAccept([]byte(key))
	if err != nil {
		return err
	}

	respHeaders := fmt.Sprintf(`HTTP/1.1 101 Switching Protocols
Connection: Upgrade
Upgrade: websocket
Sec-WebSocket-Accept: %s

`, expectedAccept)

	respHeaders = strings.ReplaceAll(respHeaders, "\n", "\r\n")
	if err := utils.WriteFull(proto.conn, []byte(respHeaders)); err != nil {
		return err
	}
	return nil
}

type WSMessage struct {
	*RawMessage
}

func containsHeaderValue(resp, headerName, value string) bool {
	for _, line := range strings.Split(resp, "\r\n") {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 && strings.EqualFold(strings.TrimSpace(parts[0]), headerName) {
			if strings.TrimSpace(parts[1]) == value {
				return true
			}
		}
	}
	return false
}

func generateNonce() ([]byte, error) {
	key := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, err
	}
	nonce := make([]byte, 24)
	base64.StdEncoding.Encode(nonce, key)
	return nonce, nil
}

func getNonceAccept(nonce []byte) (expected []byte, err error) {
	h := sha1.New()
	if _, err = h.Write(nonce); err != nil {
		return
	}
	if _, err = h.Write([]byte(websocketGUID)); err != nil {
		return
	}
	expected = make([]byte, 28)
	base64.StdEncoding.Encode(expected, h.Sum(nil))
	return
}
