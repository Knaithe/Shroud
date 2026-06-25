package utils

import (
	"crypto/md5"
	crand "crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"net"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/gofrs/uuid"
	"golang.org/x/text/encoding/simplifiedchinese"
)

func EnableKeepAlive(conn net.Conn) {
	for i := 0; i < 10; i++ {
		switch c := conn.(type) {
		case *net.TCPConn:
			c.SetKeepAlive(true)
			c.SetKeepAlivePeriod(30 * time.Second)
			return
		case interface{ NetConn() net.Conn }:
			inner := c.NetConn()
			if inner == conn {
				return
			}
			conn = inner
		default:
			return
		}
	}
}

func GenerateUUID() string {
	u2, _ := uuid.NewV4()
	uu := strings.Replace(u2.String(), "-", "", -1)
	uuid := uu[11:21]
	return uuid
}

func DeriveUUID(secret []byte, purpose string) string {
	if len(secret) == 0 {
		return GenerateUUID()
	}
	h := sha256.Sum256(append(secret, []byte(purpose)...))
	return hex.EncodeToString(h[:5])
}

func GetStringMd5(s string) string {
	h := md5.New()
	h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))
}

func GetStringSha256(s string) string {
	h := sha256.New()
	h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))
}

func StringSliceReverse(src []string) {
	if src == nil {
		return
	}
	count := len(src)
	mid := count / 2
	for i := 0; i < mid; i++ {
		tmp := src[i]
		src[i] = src[count-1]
		src[count-1] = tmp
		count--
	}
}

func Str2Int(str string) (int, error) {
	num, err := strconv.ParseInt(str, 10, 32)
	return int(uint32(num)), err
}

func Int2Str(num int) string {
	b := strconv.Itoa(num)
	return b
}

func CheckSystem() (sysType uint32) {
	var os = runtime.GOOS
	switch os {
	case "windows":
		sysType = 0x01
	case "linux":
		sysType = 0x02
	default:
		sysType = 0x03
	}
	return
}

func GetSystemInfo() (string, string) {
	var os = runtime.GOOS
	switch os {
	case "windows":
		fallthrough
	case "linux":
		fallthrough
	case "darwin":
		hostname, err := exec.Command("hostname").Output()
		if err != nil {
			hostname = []byte("Null")
		}
		username, err := exec.Command("whoami").Output()
		if err != nil {
			username = []byte("Null")
		}

		fHostname := strings.TrimRight(string(hostname), " \t\r\n")
		fUsername := strings.TrimRight(string(username), " \t\r\n")

		return fHostname, fUsername
	default:
		return "NULL", "NULL"
	}
}

func CheckIPPort(info string) (normalAddr string, reuseAddr string, err error) {
	var (
		readyIP   string
		readyPort int
	)

	spliltedInfo := strings.Split(info, ":")

	if len(spliltedInfo) == 1 {
		readyIP = "127.0.0.1"
		readyPort, err = strconv.Atoi(info)
	} else if len(spliltedInfo) == 2 {
		readyIP = spliltedInfo[0]
		readyPort, err = strconv.Atoi(spliltedInfo[1])
	} else {
		err = errors.New("please input either port(1~65535) or ip:port(1-65535)")
		return
	}

	if err != nil || readyPort < 1 || readyPort > 65535 || readyIP == "" {
		err = errors.New("please input either port(1~65535) or ip:port(1-65535)")
		return
	}

	normalAddr = readyIP + ":" + strconv.Itoa(readyPort)
	reuseAddr = "0.0.0.0:" + strconv.Itoa(readyPort)

	return
}

func CheckIfIP4(ip string) bool {
	for i := 0; i < len(ip); i++ {
		switch ip[i] {
		case '.':
			return true
		case ':':
			return false
		}
	}
	return false
}

func CheckRange(nodes []int) {
	for m := len(nodes) - 1; m > 0; m-- {
		var flag bool = false
		for n := 0; n < m; n++ {
			if nodes[n] > nodes[n+1] {
				temp := nodes[n]
				nodes[n] = nodes[n+1]
				nodes[n+1] = temp
				flag = true
			}
		}
		if !flag {
			break
		}
	}
}

func GetDigitLen(num int) int {
	var length int
	for {
		num = num / 10
		if num != 0 {
			length++
		} else {
			length++
			return length
		}
	}
}

func GetRandomString(l int) string {
	const charset = "0123456789abcdefghijklmnopqrstuvwxyz"
	result := make([]byte, l)
	for i := range result {
		var b [1]byte
		crand.Read(b[:])
		result[i] = charset[int(b[0])%len(charset)]
	}
	return string(result)
}

func GetRandomInt(max int) int {
	var buf [8]byte
	crand.Read(buf[:])
	return int(binary.BigEndian.Uint64(buf[:]) % uint64(max))
}

func ParseFileCommand(commands []string) (string, string, error) {
	if len(commands) == 2 {
		return commands[0], commands[1], nil
	} else if len(commands) > 2 {
		var count int
		full := strings.Join(commands, " ")

		for _, char := range full {
			if char == '"' {
				count++
			}
		}

		if count > 0 && count%2 == 0 {
			var final []string
			for _, part := range strings.Split(full, "\"") {
				if ready := strings.Trim(part, " \t\r\n"); ready != "" {
					final = append(final, ready)
				}
			}

			if len(final) == 2 {
				return final[0], final[1], nil
			} else {
				return "", "", errors.New("invalid format")
			}
		} else {
			return "", "", errors.New("invalid format")
		}
	}

	return "", "", errors.New("not enough arguments")
}

func ConvertStr2GBK(str string) string {
	ret, err := simplifiedchinese.GBK.NewEncoder().String(str)
	if err != nil {
		ret = str
	}
	return ret
}

func ConvertGBK2Str(gbkStr string) string {
	ret, err := simplifiedchinese.GBK.NewDecoder().String(gbkStr)
	if err != nil {
		ret = gbkStr
	}
	return ret
}

func WriteFull(conn net.Conn, data []byte) error {
	for len(data) > 0 {
		n, err := conn.Write(data)
		if err != nil {
			return err
		}
		data = data[n:]
	}
	return nil
}

func SafeSend(ch chan []byte, data []byte) (ok bool) {
	defer func() {
		if r := recover(); r != nil {
			ok = false
		}
	}()
	ch <- data
	return true
}
