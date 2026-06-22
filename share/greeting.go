package share

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

var greetHello = "Shhh..."
var greetAck = "Keep silent"

func InitGreetings(secret []byte) {
	if len(secret) == 0 {
		return
	}
	greetHello = deriveGreeting(secret, "hello")
	greetAck = deriveGreeting(secret, "ack")
}

func GreetHello() string { return greetHello }
func GreetAck() string   { return greetAck }

func deriveGreeting(secret []byte, purpose string) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte("shroud-greet-" + purpose))
	return hex.EncodeToString(mac.Sum(nil)[:8])
}
