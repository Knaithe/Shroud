package share

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"
)

type TorControl struct {
	addr     string
	password string
	conn     net.Conn
	reader   *bufio.Reader
}

func NewTorControl(addr, password string) *TorControl {
	return &TorControl{
		addr:     addr,
		password: password,
	}
}

func (tc *TorControl) Connect() error {
	conn, err := net.DialTimeout("tcp", tc.addr, 10*time.Second)
	if err != nil {
		return fmt.Errorf("cannot connect to Tor control port %s: %w", tc.addr, err)
	}
	tc.conn = conn
	tc.reader = bufio.NewReader(conn)
	return nil
}

func (tc *TorControl) Authenticate() error {
	var cmd string
	if tc.password != "" {
		cmd = fmt.Sprintf("AUTHENTICATE \"%s\"\r\n", tc.password)
	} else {
		cmd = "AUTHENTICATE\r\n"
	}

	resp, err := tc.sendCommand(cmd)
	if err != nil {
		return err
	}
	if !strings.HasPrefix(resp, "250") {
		return fmt.Errorf("authentication failed: %s", resp)
	}
	return nil
}

func (tc *TorControl) SignalNewnym() error {
	resp, err := tc.sendCommand("SIGNAL NEWNYM\r\n")
	if err != nil {
		return err
	}
	if !strings.HasPrefix(resp, "250") {
		return fmt.Errorf("NEWNYM failed: %s", resp)
	}
	return nil
}

func (tc *TorControl) GetInfo(key string) (string, error) {
	resp, err := tc.sendCommand(fmt.Sprintf("GETINFO %s\r\n", key))
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(resp, "250") {
		return "", fmt.Errorf("GETINFO failed: %s", resp)
	}
	parts := strings.SplitN(resp, "=", 2)
	if len(parts) == 2 {
		return strings.TrimSpace(parts[1]), nil
	}
	return resp, nil
}

func (tc *TorControl) AddOnion(virtPort int, targetPort int) (string, error) {
	cmd := fmt.Sprintf("ADD_ONION NEW:BEST Flags=DiscardPK Port=%d,127.0.0.1:%d\r\n", virtPort, targetPort)
	resp, err := tc.sendCommand(cmd)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(resp, "250") {
		return "", fmt.Errorf("ADD_ONION failed: %s", resp)
	}

	for _, line := range strings.Split(resp, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "250-ServiceID=") {
			serviceID := strings.TrimPrefix(line, "250-ServiceID=")
			return serviceID + ".onion", nil
		}
	}
	return "", errors.New("no ServiceID in ADD_ONION response")
}

func (tc *TorControl) AddOnionWithKey(virtPort int, targetPort int, keyType, keyBlob string) (string, error) {
	cmd := fmt.Sprintf("ADD_ONION %s:%s Port=%d,127.0.0.1:%d\r\n", keyType, keyBlob, virtPort, targetPort)
	resp, err := tc.sendCommand(cmd)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(resp, "250") {
		return "", fmt.Errorf("ADD_ONION failed: %s", resp)
	}

	for _, line := range strings.Split(resp, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "250-ServiceID=") {
			serviceID := strings.TrimPrefix(line, "250-ServiceID=")
			return serviceID + ".onion", nil
		}
	}
	return "", errors.New("no ServiceID in ADD_ONION response")
}

func (tc *TorControl) DelOnion(serviceID string) error {
	serviceID = strings.TrimSuffix(serviceID, ".onion")
	resp, err := tc.sendCommand(fmt.Sprintf("DEL_ONION %s\r\n", serviceID))
	if err != nil {
		return err
	}
	if !strings.HasPrefix(resp, "250") {
		return fmt.Errorf("DEL_ONION failed: %s", resp)
	}
	return nil
}

func (tc *TorControl) Close() {
	if tc.conn != nil {
		tc.sendCommand("QUIT\r\n")
		tc.conn.Close()
		tc.conn = nil
	}
}

func (tc *TorControl) sendCommand(cmd string) (string, error) {
	if tc.conn == nil {
		return "", errors.New("not connected to Tor control port")
	}

	_, err := tc.conn.Write([]byte(cmd))
	if err != nil {
		return "", err
	}

	var result strings.Builder
	for {
		line, err := tc.reader.ReadString('\n')
		if err != nil {
			return result.String(), err
		}
		result.WriteString(line)
		line = strings.TrimRight(line, "\r\n")
		// "250 OK" or "250 " prefix = final line; "250-..." = continuation
		if len(line) >= 4 && line[3] == ' ' {
			break
		}
	}
	return result.String(), nil
}
