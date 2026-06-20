package protocol

import (
	"bytes"
	"io"
	"net"
	"testing"
	"time"

	"Shroud/crypto"
)

const testSecret = "test-shared-secret"

// newTestRawMessage creates a RawMessage for testing with proper crypto key derivation.
func newTestRawMessage(conn net.Conn, uuid string) *RawMessage {
	return &RawMessage{
		Conn:         conn,
		UUID:         uuid,
		CryptoSecret: crypto.DeriveKey([]byte(testSecret), crypto.PurposeEncrypt),
	}
}

func TestConstructAndDeconstructData(t *testing.T) {
	t.Run("HIMess roundtrip", func(t *testing.T) {
		sender := "IAMADMINXD"   // 10 bytes
		accepter := "IAMNEWHERE" // 10 bytes
		route := "some-route"

		header := &Header{
			Sender:      sender,
			Accepter:    accepter,
			MessageType: HI,
			RouteLen:    uint32(len(route)),
			Route:       route,
		}

		hiMsg := &HIMess{
			GreetingLen: 5,
			Greeting:    "hello",
			UUIDLen:     10,
			UUID:        "ABCDEFGHIJ",
			IsAdmin:     1,
			IsReconnect: 0,
		}

		clientConn, serverConn := net.Pipe()
		defer clientConn.Close()
		defer serverConn.Close()

		senderMsg := newTestRawMessage(clientConn, sender)
		receiverMsg := newTestRawMessage(serverConn, accepter)

		// Construct and send on client side
		senderMsg.ConstructData(header, hiMsg, false)
		go senderMsg.SendMessage()

		// Deconstruct on server side
		gotHeader, gotMess, err := receiverMsg.DeconstructData()
		if err != nil {
			t.Fatalf("DeconstructData() error: %v", err)
		}

		if gotHeader.Sender != sender {
			t.Errorf("sender: want %q, got %q", sender, gotHeader.Sender)
		}
		if gotHeader.Accepter != accepter {
			t.Errorf("accepter: want %q, got %q", accepter, gotHeader.Accepter)
		}
		if gotHeader.MessageType != HI {
			t.Errorf("message type: want %d, got %d", HI, gotHeader.MessageType)
		}
		if gotHeader.Route != route {
			t.Errorf("route: want %q, got %q", route, gotHeader.Route)
		}

		gotHI, ok := gotMess.(*HIMess)
		if !ok {
			t.Fatalf("expected *HIMess, got %T", gotMess)
		}
		if gotHI.Greeting != "hello" {
			t.Errorf("greeting: want %q, got %q", "hello", gotHI.Greeting)
		}
		if gotHI.UUID != "ABCDEFGHIJ" {
			t.Errorf("uuid: want %q, got %q", "ABCDEFGHIJ", gotHI.UUID)
		}
		if gotHI.IsAdmin != 1 {
			t.Errorf("isAdmin: want 1, got %d", gotHI.IsAdmin)
		}
		if gotHI.IsReconnect != 0 {
			t.Errorf("isReconnect: want 0, got %d", gotHI.IsReconnect)
		}
	})

	t.Run("UUIDMess roundtrip", func(t *testing.T) {
		sender := "IAMADMINXD"
		accepter := "IAMNEWHERE"
		route := ""

		header := &Header{
			Sender:      sender,
			Accepter:    accepter,
			MessageType: UUID,
			RouteLen:    uint32(len(route)),
			Route:       route,
		}

		uuidMsg := &UUIDMess{
			UUIDLen: 10,
			UUID:    "TESTUUID01",
		}

		clientConn, serverConn := net.Pipe()
		defer clientConn.Close()
		defer serverConn.Close()

		senderMsg := newTestRawMessage(clientConn, sender)
		receiverMsg := newTestRawMessage(serverConn, accepter)

		senderMsg.ConstructData(header, uuidMsg, false)
		go senderMsg.SendMessage()

		gotHeader, gotMess, err := receiverMsg.DeconstructData()
		if err != nil {
			t.Fatalf("DeconstructData() error: %v", err)
		}

		if gotHeader.MessageType != UUID {
			t.Errorf("message type: want %d, got %d", UUID, gotHeader.MessageType)
		}

		gotUUID, ok := gotMess.(*UUIDMess)
		if !ok {
			t.Fatalf("expected *UUIDMess, got %T", gotMess)
		}
		if gotUUID.UUID != "TESTUUID01" {
			t.Errorf("uuid: want %q, got %q", "TESTUUID01", gotUUID.UUID)
		}
	})
}

func TestSendMessage(t *testing.T) {
	t.Run("data is written to connection", func(t *testing.T) {
		clientConn, serverConn := net.Pipe()
		defer clientConn.Close()
		defer serverConn.Close()

		msg := newTestRawMessage(clientConn, ADMIN_UUID)

		header := &Header{
			Sender:      ADMIN_UUID,
			Accepter:    TEMP_UUID,
			MessageType: HEARTBEAT,
			RouteLen:    0,
			Route:       "",
		}

		heartbeat := &HeartbeatMsg{Ping: 1}
		msg.ConstructData(header, heartbeat, false)

		// SendMessage in background
		go msg.SendMessage()

		// Read from the other end -- should get some bytes
		serverConn.SetReadDeadline(time.Now().Add(5 * time.Second))
		buf := make([]byte, 8192)
		n, err := serverConn.Read(buf)
		if err != nil {
			t.Fatalf("Read() error: %v", err)
		}
		if n == 0 {
			t.Fatal("expected data to be written to connection, got 0 bytes")
		}
		// Header is: sender(10) + accepter(10) + msgType(2) + routeLen(4) + route(0) + dataLen(8) = 34
		// Plus encrypted+compressed data
		if n < 34 {
			t.Fatalf("expected at least 34 bytes of header, got %d", n)
		}
	})

	t.Run("buffers cleared after send", func(t *testing.T) {
		clientConn, serverConn := net.Pipe()
		defer clientConn.Close()
		defer serverConn.Close()

		msg := newTestRawMessage(clientConn, ADMIN_UUID)

		header := &Header{
			Sender:      ADMIN_UUID,
			Accepter:    TEMP_UUID,
			MessageType: HEARTBEAT,
			RouteLen:    0,
			Route:       "",
		}

		heartbeat := &HeartbeatMsg{Ping: 1}
		msg.ConstructData(header, heartbeat, false)

		// Drain on the server side
		go func() {
			io.Copy(io.Discard, serverConn)
		}()

		msg.SendMessage()

		if msg.HeaderBuffer != nil {
			t.Error("HeaderBuffer should be nil after SendMessage()")
		}
		if msg.DataBuffer != nil {
			t.Error("DataBuffer should be nil after SendMessage()")
		}
	})
}

func TestPassthroughMode(t *testing.T) {
	t.Run("isPass true sends raw bytes without encryption", func(t *testing.T) {
		clientConn, serverConn := net.Pipe()
		defer clientConn.Close()
		defer serverConn.Close()

		sender := "IAMADMINXD"
		// Use an accepter that is NOT TEMP_UUID and NOT the receiver's own UUID,
		// so DeconstructData treats the payload as passthrough (returns raw bytes).
		accepter := "OTHERNODEx"

		header := &Header{
			Sender:      sender,
			Accepter:    accepter,
			MessageType: HI,
			RouteLen:    0,
			Route:       "",
		}

		rawPayload := []byte{0xDE, 0xAD, 0xBE, 0xEF, 0x01, 0x02, 0x03, 0x04}

		senderMsg := newTestRawMessage(clientConn, sender)
		senderMsg.ConstructData(header, rawPayload, true)

		// Receiver with a UUID that doesn't match accepter and isn't ADMIN_UUID
		receiverMsg := newTestRawMessage(serverConn, "RECEIVERXX")

		go senderMsg.SendMessage()

		gotHeader, gotMess, err := receiverMsg.DeconstructData()
		if err != nil {
			t.Fatalf("DeconstructData() error: %v", err)
		}

		if gotHeader.MessageType != HI {
			t.Errorf("message type: want %d, got %d", HI, gotHeader.MessageType)
		}

		// In passthrough mode, data is returned as []byte
		gotBytes, ok := gotMess.([]byte)
		if !ok {
			t.Fatalf("expected []byte in passthrough mode, got %T", gotMess)
		}

		// The raw payload was NOT encrypted, so the returned bytes should be exactly
		// what was sent (since isPass=true skips encrypt/compress on construct).
		if len(gotBytes) != len(rawPayload) {
			t.Fatalf("payload length: want %d, got %d", len(rawPayload), len(gotBytes))
		}
		for i := range rawPayload {
			if gotBytes[i] != rawPayload[i] {
				t.Errorf("byte %d: want 0x%02x, got 0x%02x", i, rawPayload[i], gotBytes[i])
			}
		}
	})
}

func TestConstructMessageAndDestructMessage(t *testing.T) {
	t.Run("high level helpers roundtrip", func(t *testing.T) {
		clientConn, serverConn := net.Pipe()
		defer clientConn.Close()
		defer serverConn.Close()

		sender := ADMIN_UUID
		accepter := TEMP_UUID
		route := "node1:node2"

		header := &Header{
			Sender:      sender,
			Accepter:    accepter,
			MessageType: SHELLCOMMAND,
			RouteLen:    uint32(len(route)),
			Route:       route,
		}

		shellCmd := &ShellCommand{
			CommandLen: 11,
			Command:    "cat /etc/os",
		}

		senderMsg := newTestRawMessage(clientConn, sender)
		receiverMsg := newTestRawMessage(serverConn, accepter)

		ConstructMessage(senderMsg, header, shellCmd, false)

		go senderMsg.SendMessage()

		gotHeader, gotMess, err := DestructMessage(receiverMsg)
		if err != nil {
			t.Fatalf("DestructMessage() error: %v", err)
		}

		if gotHeader.Sender != sender {
			t.Errorf("sender: want %q, got %q", sender, gotHeader.Sender)
		}
		if gotHeader.Accepter != accepter {
			t.Errorf("accepter: want %q, got %q", accepter, gotHeader.Accepter)
		}
		if gotHeader.MessageType != SHELLCOMMAND {
			t.Errorf("message type: want %d, got %d", SHELLCOMMAND, gotHeader.MessageType)
		}
		if gotHeader.Route != route {
			t.Errorf("route: want %q, got %q", route, gotHeader.Route)
		}

		gotShell, ok := gotMess.(*ShellCommand)
		if !ok {
			t.Fatalf("expected *ShellCommand, got %T", gotMess)
		}
		if gotShell.Command != "cat /etc/os" {
			t.Errorf("command: want %q, got %q", "cat /etc/os", gotShell.Command)
		}
	})

	t.Run("high level helpers passthrough roundtrip", func(t *testing.T) {
		clientConn, serverConn := net.Pipe()
		defer clientConn.Close()
		defer serverConn.Close()

		sender := "IAMADMINXD"
		accepter := "OTHERNODEx"

		header := &Header{
			Sender:      sender,
			Accepter:    accepter,
			MessageType: FORWARDDATA,
			RouteLen:    0,
			Route:       "",
		}

		rawData := []byte("passthrough-test-payload-1234567890")

		senderMsg := newTestRawMessage(clientConn, sender)
		receiverMsg := newTestRawMessage(serverConn, "RECEIVERXX")

		ConstructMessage(senderMsg, header, rawData, true)

		go senderMsg.SendMessage()

		gotHeader, gotMess, err := DestructMessage(receiverMsg)
		if err != nil {
			t.Fatalf("DestructMessage() error: %v", err)
		}

		if gotHeader.MessageType != FORWARDDATA {
			t.Errorf("message type: want %d, got %d", FORWARDDATA, gotHeader.MessageType)
		}

		gotBytes, ok := gotMess.([]byte)
		if !ok {
			t.Fatalf("expected []byte for passthrough, got %T", gotMess)
		}
		if string(gotBytes) != string(rawData) {
			t.Errorf("data: want %q, got %q", rawData, gotBytes)
		}
	})
}

func TestMultipleMessagesOnSameConnection(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	sender := ADMIN_UUID
	accepter := TEMP_UUID

	go func() {
		for i := 0; i < 3; i++ {
			msg := newTestRawMessage(clientConn, sender)
			header := &Header{
				Sender:      sender,
				Accepter:    accepter,
				MessageType: HEARTBEAT,
				RouteLen:    0,
				Route:       "",
			}
			hb := &HeartbeatMsg{Ping: uint16(i + 1)}
			ConstructMessage(msg, header, hb, false)
			msg.SendMessage()
		}
	}()

	for i := 0; i < 3; i++ {
		recvMsg := newTestRawMessage(serverConn, accepter)
		gotHeader, gotMess, err := DestructMessage(recvMsg)
		if err != nil {
			t.Fatalf("message %d: DestructMessage() error: %v", i, err)
		}
		if gotHeader.MessageType != HEARTBEAT {
			t.Errorf("message %d: type want %d, got %d", i, HEARTBEAT, gotHeader.MessageType)
		}
		gotHB, ok := gotMess.(*HeartbeatMsg)
		if !ok {
			t.Fatalf("message %d: expected *HeartbeatMsg, got %T", i, gotMess)
		}
		if gotHB.Ping != uint16(i+1) {
			t.Errorf("message %d: ping want %d, got %d", i, i+1, gotHB.Ping)
		}
	}
}

func TestE2EKeyPreventsIntermediatePayloadDecode(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	header := &Header{
		Sender:      ADMIN_UUID,
		Accepter:    "AGENT00001",
		MessageType: SHELLCOMMAND,
		RouteLen:    0,
		Route:       "",
	}
	shellCmd := &ShellCommand{CommandLen: 6, Command: "whoami"}
	key := bytes.Repeat([]byte{0x42}, 32)

	senderMsg := newTestRawMessage(clientConn, ADMIN_UUID)
	senderMsg.E2EKey = key
	targetMsg := newTestRawMessage(serverConn, "AGENT00001")
	targetMsg.E2EKey = key
	ConstructMessage(senderMsg, header, shellCmd, false)
	go senderMsg.SendMessage()
	gotHeader, gotMess, err := DestructMessage(targetMsg)
	if err != nil {
		t.Fatalf("target decrypt failed: %v", err)
	}
	if gotHeader.MessageType != SHELLCOMMAND {
		t.Fatalf("wrong message type: %d", gotHeader.MessageType)
	}
	gotCmd, ok := gotMess.(*ShellCommand)
	if !ok || gotCmd.Command != "whoami" {
		t.Fatalf("unexpected command %#v", gotMess)
	}

	clientConn2, serverConn2 := net.Pipe()
	defer clientConn2.Close()
	defer serverConn2.Close()
	senderMsg2 := newTestRawMessage(clientConn2, ADMIN_UUID)
	senderMsg2.E2EKey = key
	middleMsg := newTestRawMessage(serverConn2, "MIDDLE0001")
	middleMsg.E2EKey = bytes.Repeat([]byte{0x24}, 32)
	ConstructMessage(senderMsg2, header, shellCmd, false)
	go senderMsg2.SendMessage()
	_, gotMiddle, err := DestructMessage(middleMsg)
	if err != nil {
		t.Fatalf("middle passthrough failed: %v", err)
	}
	if _, ok := gotMiddle.(*ShellCommand); ok {
		t.Fatal("intermediate decoded target command")
	}
	gotRaw, ok := gotMiddle.([]byte)
	if !ok {
		t.Fatalf("expected encrypted passthrough bytes, got %T", gotMiddle)
	}
	if bytes.Contains(gotRaw, []byte("whoami")) {
		t.Fatal("passthrough bytes contain plaintext command")
	}
}
