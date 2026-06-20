package protocol

import (
	"bytes"
	"testing"
)

func TestHIMess_Roundtrip(t *testing.T) {
	orig := HIMess{
		GreetingLen: 13,
		Greeting:    "Hello, Shroud",
		UUIDLen:     10,
		UUID:        "ABCDEFGHIJ",
		IsAdmin:     1,
		IsReconnect: 0,
	}

	data := orig.MarshalBinary()

	var got HIMess
	if err := got.UnmarshalBinary(data); err != nil {
		t.Fatalf("UnmarshalBinary failed: %v", err)
	}

	if got.GreetingLen != orig.GreetingLen {
		t.Errorf("GreetingLen: got %d, want %d", got.GreetingLen, orig.GreetingLen)
	}
	if got.Greeting != orig.Greeting {
		t.Errorf("Greeting: got %q, want %q", got.Greeting, orig.Greeting)
	}
	if got.UUIDLen != orig.UUIDLen {
		t.Errorf("UUIDLen: got %d, want %d", got.UUIDLen, orig.UUIDLen)
	}
	if got.UUID != orig.UUID {
		t.Errorf("UUID: got %q, want %q", got.UUID, orig.UUID)
	}
	if got.IsAdmin != orig.IsAdmin {
		t.Errorf("IsAdmin: got %d, want %d", got.IsAdmin, orig.IsAdmin)
	}
	if got.IsReconnect != orig.IsReconnect {
		t.Errorf("IsReconnect: got %d, want %d", got.IsReconnect, orig.IsReconnect)
	}
}

func TestSSHReq_Roundtrip(t *testing.T) {
	cert := []byte{0x30, 0x82, 0x01, 0x22, 0x30, 0x0d, 0x06, 0x09}

	orig := SSHReq{
		Method:                1,
		AddrLen:               14,
		Addr:                  "10.0.0.1:22022",
		UsernameLen:           4,
		Username:              "root",
		PasswordLen:           12,
		Password:              "hunter2admin",
		CertificateLen:        uint64(len(cert)),
		Certificate:           cert,
		HostKeyFingerprintLen: 50,
		HostKeyFingerprint:    "SHA256:nThbg6kXUpJWGl7E1IGOCspRomTxdCARLviKw6E5SY8",
	}

	data := orig.MarshalBinary()

	var got SSHReq
	if err := got.UnmarshalBinary(data); err != nil {
		t.Fatalf("UnmarshalBinary failed: %v", err)
	}

	if got.Method != orig.Method {
		t.Errorf("Method: got %d, want %d", got.Method, orig.Method)
	}
	if got.AddrLen != orig.AddrLen {
		t.Errorf("AddrLen: got %d, want %d", got.AddrLen, orig.AddrLen)
	}
	if got.Addr != orig.Addr {
		t.Errorf("Addr: got %q, want %q", got.Addr, orig.Addr)
	}
	if got.UsernameLen != orig.UsernameLen {
		t.Errorf("UsernameLen: got %d, want %d", got.UsernameLen, orig.UsernameLen)
	}
	if got.Username != orig.Username {
		t.Errorf("Username: got %q, want %q", got.Username, orig.Username)
	}
	if got.PasswordLen != orig.PasswordLen {
		t.Errorf("PasswordLen: got %d, want %d", got.PasswordLen, orig.PasswordLen)
	}
	if got.Password != orig.Password {
		t.Errorf("Password: got %q, want %q", got.Password, orig.Password)
	}
	if got.CertificateLen != orig.CertificateLen {
		t.Errorf("CertificateLen: got %d, want %d", got.CertificateLen, orig.CertificateLen)
	}
	if !bytes.Equal(got.Certificate, orig.Certificate) {
		t.Errorf("Certificate: got %x, want %x", got.Certificate, orig.Certificate)
	}
	if got.HostKeyFingerprintLen != orig.HostKeyFingerprintLen {
		t.Errorf("HostKeyFingerprintLen: got %d, want %d", got.HostKeyFingerprintLen, orig.HostKeyFingerprintLen)
	}
	if got.HostKeyFingerprint != orig.HostKeyFingerprint {
		t.Errorf("HostKeyFingerprint: got %q, want %q", got.HostKeyFingerprint, orig.HostKeyFingerprint)
	}
}

func TestSSHTunnelReq_Roundtrip(t *testing.T) {
	cert := []byte{0xDE, 0xAD, 0xBE, 0xEF, 0xCA, 0xFE}

	orig := SSHTunnelReq{
		Method:                2,
		AddrLen:               15,
		Addr:                  "192.168.1.1:443",
		PortLen:               4,
		Port:                  "8080",
		UsernameLen:           5,
		Username:              "admin",
		PasswordLen:           8,
		Password:              "p@ssw0rd",
		CertificateLen:        uint64(len(cert)),
		Certificate:           cert,
		HostKeyFingerprintLen: 49,
		HostKeyFingerprint:    "SHA256:xyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789abc",
	}

	data := orig.MarshalBinary()

	var got SSHTunnelReq
	if err := got.UnmarshalBinary(data); err != nil {
		t.Fatalf("UnmarshalBinary failed: %v", err)
	}

	if got.Method != orig.Method {
		t.Errorf("Method: got %d, want %d", got.Method, orig.Method)
	}
	if got.AddrLen != orig.AddrLen {
		t.Errorf("AddrLen: got %d, want %d", got.AddrLen, orig.AddrLen)
	}
	if got.Addr != orig.Addr {
		t.Errorf("Addr: got %q, want %q", got.Addr, orig.Addr)
	}
	if got.PortLen != orig.PortLen {
		t.Errorf("PortLen: got %d, want %d", got.PortLen, orig.PortLen)
	}
	if got.Port != orig.Port {
		t.Errorf("Port: got %q, want %q", got.Port, orig.Port)
	}
	if got.UsernameLen != orig.UsernameLen {
		t.Errorf("UsernameLen: got %d, want %d", got.UsernameLen, orig.UsernameLen)
	}
	if got.Username != orig.Username {
		t.Errorf("Username: got %q, want %q", got.Username, orig.Username)
	}
	if got.PasswordLen != orig.PasswordLen {
		t.Errorf("PasswordLen: got %d, want %d", got.PasswordLen, orig.PasswordLen)
	}
	if got.Password != orig.Password {
		t.Errorf("Password: got %q, want %q", got.Password, orig.Password)
	}
	if got.CertificateLen != orig.CertificateLen {
		t.Errorf("CertificateLen: got %d, want %d", got.CertificateLen, orig.CertificateLen)
	}
	if !bytes.Equal(got.Certificate, orig.Certificate) {
		t.Errorf("Certificate: got %x, want %x", got.Certificate, orig.Certificate)
	}
	if got.HostKeyFingerprintLen != orig.HostKeyFingerprintLen {
		t.Errorf("HostKeyFingerprintLen: got %d, want %d", got.HostKeyFingerprintLen, orig.HostKeyFingerprintLen)
	}
	if got.HostKeyFingerprint != orig.HostKeyFingerprint {
		t.Errorf("HostKeyFingerprint: got %q, want %q", got.HostKeyFingerprint, orig.HostKeyFingerprint)
	}
}

func TestFileStatReq_Roundtrip(t *testing.T) {
	orig := FileStatReq{
		TransferID:  0x00AABBCCDDEEFF11,
		FilenameLen: 16,
		Filename:    "secret_data.docx",
		FileSize:    1048576,
		SliceNum:    64,
	}

	data := orig.MarshalBinary()

	var got FileStatReq
	if err := got.UnmarshalBinary(data); err != nil {
		t.Fatalf("UnmarshalBinary failed: %v", err)
	}

	if got.TransferID != orig.TransferID {
		t.Errorf("TransferID: got %d, want %d", got.TransferID, orig.TransferID)
	}
	if got.FilenameLen != orig.FilenameLen {
		t.Errorf("FilenameLen: got %d, want %d", got.FilenameLen, orig.FilenameLen)
	}
	if got.Filename != orig.Filename {
		t.Errorf("Filename: got %q, want %q", got.Filename, orig.Filename)
	}
	if got.FileSize != orig.FileSize {
		t.Errorf("FileSize: got %d, want %d", got.FileSize, orig.FileSize)
	}
	if got.SliceNum != orig.SliceNum {
		t.Errorf("SliceNum: got %d, want %d", got.SliceNum, orig.SliceNum)
	}
}

func TestFileData_Roundtrip(t *testing.T) {
	payload := []byte{
		0x50, 0x4B, 0x03, 0x04, 0x14, 0x00, 0x06, 0x00,
		0x08, 0x00, 0x00, 0x00, 0x21, 0x00, 0xF0, 0x74,
		0xFF, 0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0x01, 0x02,
	}

	orig := FileData{
		TransferID: 42,
		DataLen:    uint64(len(payload)),
		Data:       payload,
	}

	data := orig.MarshalBinary()

	var got FileData
	if err := got.UnmarshalBinary(data); err != nil {
		t.Fatalf("UnmarshalBinary failed: %v", err)
	}

	if got.TransferID != orig.TransferID {
		t.Errorf("TransferID: got %d, want %d", got.TransferID, orig.TransferID)
	}
	if got.DataLen != orig.DataLen {
		t.Errorf("DataLen: got %d, want %d", got.DataLen, orig.DataLen)
	}
	if !bytes.Equal(got.Data, orig.Data) {
		t.Errorf("Data: got %x, want %x", got.Data, orig.Data)
	}
}

func TestSocksTCPData_Roundtrip(t *testing.T) {
	payload := []byte("GET / HTTP/1.1\r\nHost: example.com\r\n\r\n")

	orig := SocksTCPData{
		Seq:     9001,
		DataLen: uint64(len(payload)),
		Data:    payload,
	}

	data := orig.MarshalBinary()

	var got SocksTCPData
	if err := got.UnmarshalBinary(data); err != nil {
		t.Fatalf("UnmarshalBinary failed: %v", err)
	}

	if got.Seq != orig.Seq {
		t.Errorf("Seq: got %d, want %d", got.Seq, orig.Seq)
	}
	if got.DataLen != orig.DataLen {
		t.Errorf("DataLen: got %d, want %d", got.DataLen, orig.DataLen)
	}
	if !bytes.Equal(got.Data, orig.Data) {
		t.Errorf("Data: got %x, want %x", got.Data, orig.Data)
	}
}

func TestForwardData_Roundtrip(t *testing.T) {
	payload := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}

	orig := ForwardData{
		Seq:     256,
		DataLen: uint64(len(payload)),
		Data:    payload,
	}

	data := orig.MarshalBinary()

	var got ForwardData
	if err := got.UnmarshalBinary(data); err != nil {
		t.Fatalf("UnmarshalBinary failed: %v", err)
	}

	if got.Seq != orig.Seq {
		t.Errorf("Seq: got %d, want %d", got.Seq, orig.Seq)
	}
	if got.DataLen != orig.DataLen {
		t.Errorf("DataLen: got %d, want %d", got.DataLen, orig.DataLen)
	}
	if !bytes.Equal(got.Data, orig.Data) {
		t.Errorf("Data: got %x, want %x", got.Data, orig.Data)
	}
}

func TestBackwardData_Roundtrip(t *testing.T) {
	payload := []byte{0xCA, 0xFE, 0xBA, 0xBE, 0xDE, 0xAD, 0xBE, 0xEF, 0x00, 0x11}

	orig := BackwardData{
		Seq:     0xFFFFFFFFFFFFFFFF,
		DataLen: uint64(len(payload)),
		Data:    payload,
	}

	data := orig.MarshalBinary()

	var got BackwardData
	if err := got.UnmarshalBinary(data); err != nil {
		t.Fatalf("UnmarshalBinary failed: %v", err)
	}

	if got.Seq != orig.Seq {
		t.Errorf("Seq: got %d, want %d", got.Seq, orig.Seq)
	}
	if got.DataLen != orig.DataLen {
		t.Errorf("DataLen: got %d, want %d", got.DataLen, orig.DataLen)
	}
	if !bytes.Equal(got.Data, orig.Data) {
		t.Errorf("Data: got %x, want %x", got.Data, orig.Data)
	}
}

// ---------------------------------------------------------------------------
// Roundtrip tests for all remaining message types
// ---------------------------------------------------------------------------

func TestUUIDMess_Roundtrip(t *testing.T) {
	orig := UUIDMess{
		UUIDLen: 10,
		UUID:    "ABCDEF1234",
	}
	data := orig.MarshalBinary()
	var got UUIDMess
	if err := got.UnmarshalBinary(data); err != nil {
		t.Fatalf("UnmarshalBinary failed: %v", err)
	}
	if got.UUIDLen != orig.UUIDLen {
		t.Errorf("UUIDLen: got %d, want %d", got.UUIDLen, orig.UUIDLen)
	}
	if got.UUID != orig.UUID {
		t.Errorf("UUID: got %q, want %q", got.UUID, orig.UUID)
	}
}

func TestChildUUIDReq_Roundtrip(t *testing.T) {
	orig := ChildUUIDReq{
		ParentUUIDLen: 10,
		ParentUUID:    "PARENT0001",
		IPLen:         13,
		IP:            "192.168.1.100",
	}
	data := orig.MarshalBinary()
	var got ChildUUIDReq
	if err := got.UnmarshalBinary(data); err != nil {
		t.Fatalf("UnmarshalBinary failed: %v", err)
	}
	if got.ParentUUIDLen != orig.ParentUUIDLen {
		t.Errorf("ParentUUIDLen: got %d, want %d", got.ParentUUIDLen, orig.ParentUUIDLen)
	}
	if got.ParentUUID != orig.ParentUUID {
		t.Errorf("ParentUUID: got %q, want %q", got.ParentUUID, orig.ParentUUID)
	}
	if got.IPLen != orig.IPLen {
		t.Errorf("IPLen: got %d, want %d", got.IPLen, orig.IPLen)
	}
	if got.IP != orig.IP {
		t.Errorf("IP: got %q, want %q", got.IP, orig.IP)
	}
}

func TestChildUUIDRes_Roundtrip(t *testing.T) {
	orig := ChildUUIDRes{
		UUIDLen: 10,
		UUID:    "CHILD00001",
	}
	data := orig.MarshalBinary()
	var got ChildUUIDRes
	if err := got.UnmarshalBinary(data); err != nil {
		t.Fatalf("UnmarshalBinary failed: %v", err)
	}
	if got.UUIDLen != orig.UUIDLen {
		t.Errorf("UUIDLen: got %d, want %d", got.UUIDLen, orig.UUIDLen)
	}
	if got.UUID != orig.UUID {
		t.Errorf("UUID: got %q, want %q", got.UUID, orig.UUID)
	}
}

func TestMyInfo_Roundtrip(t *testing.T) {
	orig := MyInfo{
		UUIDLen:     10,
		UUID:        "NODE00ABCD",
		UsernameLen: 5,
		Username:    "admin",
		HostnameLen: 11,
		Hostname:    "web-server1",
		MemoLen:     17,
		Memo:        "production server",
	}
	data := orig.MarshalBinary()
	var got MyInfo
	if err := got.UnmarshalBinary(data); err != nil {
		t.Fatalf("UnmarshalBinary failed: %v", err)
	}
	if got.UUIDLen != orig.UUIDLen {
		t.Errorf("UUIDLen: got %d, want %d", got.UUIDLen, orig.UUIDLen)
	}
	if got.UUID != orig.UUID {
		t.Errorf("UUID: got %q, want %q", got.UUID, orig.UUID)
	}
	if got.UsernameLen != orig.UsernameLen {
		t.Errorf("UsernameLen: got %d, want %d", got.UsernameLen, orig.UsernameLen)
	}
	if got.Username != orig.Username {
		t.Errorf("Username: got %q, want %q", got.Username, orig.Username)
	}
	if got.HostnameLen != orig.HostnameLen {
		t.Errorf("HostnameLen: got %d, want %d", got.HostnameLen, orig.HostnameLen)
	}
	if got.Hostname != orig.Hostname {
		t.Errorf("Hostname: got %q, want %q", got.Hostname, orig.Hostname)
	}
	if got.MemoLen != orig.MemoLen {
		t.Errorf("MemoLen: got %d, want %d", got.MemoLen, orig.MemoLen)
	}
	if got.Memo != orig.Memo {
		t.Errorf("Memo: got %q, want %q", got.Memo, orig.Memo)
	}
}

func TestMyMemo_Roundtrip(t *testing.T) {
	orig := MyMemo{
		MemoLen: 14,
		Memo:    "staging server",
	}
	data := orig.MarshalBinary()
	var got MyMemo
	if err := got.UnmarshalBinary(data); err != nil {
		t.Fatalf("UnmarshalBinary failed: %v", err)
	}
	if got.MemoLen != orig.MemoLen {
		t.Errorf("MemoLen: got %d, want %d", got.MemoLen, orig.MemoLen)
	}
	if got.Memo != orig.Memo {
		t.Errorf("Memo: got %q, want %q", got.Memo, orig.Memo)
	}
}

func TestShellReq_Roundtrip(t *testing.T) {
	orig := ShellReq{
		Start: 1,
	}
	data := orig.MarshalBinary()
	var got ShellReq
	if err := got.UnmarshalBinary(data); err != nil {
		t.Fatalf("UnmarshalBinary failed: %v", err)
	}
	if got.Start != orig.Start {
		t.Errorf("Start: got %d, want %d", got.Start, orig.Start)
	}
}

func TestShellRes_Roundtrip(t *testing.T) {
	orig := ShellRes{
		OK: 1,
	}
	data := orig.MarshalBinary()
	var got ShellRes
	if err := got.UnmarshalBinary(data); err != nil {
		t.Fatalf("UnmarshalBinary failed: %v", err)
	}
	if got.OK != orig.OK {
		t.Errorf("OK: got %d, want %d", got.OK, orig.OK)
	}
}

func TestShellCommand_Roundtrip(t *testing.T) {
	orig := ShellCommand{
		CommandLen: 17,
		Command:    "cat /etc/hostname",
	}
	data := orig.MarshalBinary()
	var got ShellCommand
	if err := got.UnmarshalBinary(data); err != nil {
		t.Fatalf("UnmarshalBinary failed: %v", err)
	}
	if got.CommandLen != orig.CommandLen {
		t.Errorf("CommandLen: got %d, want %d", got.CommandLen, orig.CommandLen)
	}
	if got.Command != orig.Command {
		t.Errorf("Command: got %q, want %q", got.Command, orig.Command)
	}
}

func TestShellResult_Roundtrip(t *testing.T) {
	orig := ShellResult{
		ResultLen: 11,
		Result:    "web-server1",
	}
	data := orig.MarshalBinary()
	var got ShellResult
	if err := got.UnmarshalBinary(data); err != nil {
		t.Fatalf("UnmarshalBinary failed: %v", err)
	}
	if got.ResultLen != orig.ResultLen {
		t.Errorf("ResultLen: got %d, want %d", got.ResultLen, orig.ResultLen)
	}
	if got.Result != orig.Result {
		t.Errorf("Result: got %q, want %q", got.Result, orig.Result)
	}
}

func TestShellExit_Roundtrip(t *testing.T) {
	orig := ShellExit{
		OK: 1,
	}
	data := orig.MarshalBinary()
	var got ShellExit
	if err := got.UnmarshalBinary(data); err != nil {
		t.Fatalf("UnmarshalBinary failed: %v", err)
	}
	if got.OK != orig.OK {
		t.Errorf("OK: got %d, want %d", got.OK, orig.OK)
	}
}

func TestListenReq_Roundtrip(t *testing.T) {
	orig := ListenReq{
		Method:  2,
		AddrLen: 13,
		Addr:    "0.0.0.0:10080",
	}
	data := orig.MarshalBinary()
	var got ListenReq
	if err := got.UnmarshalBinary(data); err != nil {
		t.Fatalf("UnmarshalBinary failed: %v", err)
	}
	if got.Method != orig.Method {
		t.Errorf("Method: got %d, want %d", got.Method, orig.Method)
	}
	if got.AddrLen != orig.AddrLen {
		t.Errorf("AddrLen: got %d, want %d", got.AddrLen, orig.AddrLen)
	}
	if got.Addr != orig.Addr {
		t.Errorf("Addr: got %q, want %q", got.Addr, orig.Addr)
	}
}

func TestListenRes_Roundtrip(t *testing.T) {
	orig := ListenRes{
		OK: 1,
	}
	data := orig.MarshalBinary()
	var got ListenRes
	if err := got.UnmarshalBinary(data); err != nil {
		t.Fatalf("UnmarshalBinary failed: %v", err)
	}
	if got.OK != orig.OK {
		t.Errorf("OK: got %d, want %d", got.OK, orig.OK)
	}
}

func TestSSHRes_Roundtrip(t *testing.T) {
	orig := SSHRes{
		OK: 1,
	}
	data := orig.MarshalBinary()
	var got SSHRes
	if err := got.UnmarshalBinary(data); err != nil {
		t.Fatalf("UnmarshalBinary failed: %v", err)
	}
	if got.OK != orig.OK {
		t.Errorf("OK: got %d, want %d", got.OK, orig.OK)
	}
}

func TestSSHCommand_Roundtrip(t *testing.T) {
	orig := SSHCommand{
		CommandLen: 6,
		Command:    "whoami",
	}
	data := orig.MarshalBinary()
	var got SSHCommand
	if err := got.UnmarshalBinary(data); err != nil {
		t.Fatalf("UnmarshalBinary failed: %v", err)
	}
	if got.CommandLen != orig.CommandLen {
		t.Errorf("CommandLen: got %d, want %d", got.CommandLen, orig.CommandLen)
	}
	if got.Command != orig.Command {
		t.Errorf("Command: got %q, want %q", got.Command, orig.Command)
	}
}

func TestSSHResult_Roundtrip(t *testing.T) {
	orig := SSHResult{
		ResultLen: 4,
		Result:    "root",
	}
	data := orig.MarshalBinary()
	var got SSHResult
	if err := got.UnmarshalBinary(data); err != nil {
		t.Fatalf("UnmarshalBinary failed: %v", err)
	}
	if got.ResultLen != orig.ResultLen {
		t.Errorf("ResultLen: got %d, want %d", got.ResultLen, orig.ResultLen)
	}
	if got.Result != orig.Result {
		t.Errorf("Result: got %q, want %q", got.Result, orig.Result)
	}
}

func TestSSHExit_Roundtrip(t *testing.T) {
	orig := SSHExit{
		OK: 1,
	}
	data := orig.MarshalBinary()
	var got SSHExit
	if err := got.UnmarshalBinary(data); err != nil {
		t.Fatalf("UnmarshalBinary failed: %v", err)
	}
	if got.OK != orig.OK {
		t.Errorf("OK: got %d, want %d", got.OK, orig.OK)
	}
}

func TestSSHTunnelRes_Roundtrip(t *testing.T) {
	orig := SSHTunnelRes{
		OK: 1,
	}
	data := orig.MarshalBinary()
	var got SSHTunnelRes
	if err := got.UnmarshalBinary(data); err != nil {
		t.Fatalf("UnmarshalBinary failed: %v", err)
	}
	if got.OK != orig.OK {
		t.Errorf("OK: got %d, want %d", got.OK, orig.OK)
	}
}

func TestFileStatRes_Roundtrip(t *testing.T) {
	orig := FileStatRes{
		TransferID: 0x00AABBCCDDEEFF22,
		OK:         1,
	}
	data := orig.MarshalBinary()
	var got FileStatRes
	if err := got.UnmarshalBinary(data); err != nil {
		t.Fatalf("UnmarshalBinary failed: %v", err)
	}
	if got.TransferID != orig.TransferID {
		t.Errorf("TransferID: got %d, want %d", got.TransferID, orig.TransferID)
	}
	if got.OK != orig.OK {
		t.Errorf("OK: got %d, want %d", got.OK, orig.OK)
	}
}

func TestFileErr_Roundtrip(t *testing.T) {
	orig := FileErr{
		TransferID: 12345678,
		Error:      3,
	}
	data := orig.MarshalBinary()
	var got FileErr
	if err := got.UnmarshalBinary(data); err != nil {
		t.Fatalf("UnmarshalBinary failed: %v", err)
	}
	if got.TransferID != orig.TransferID {
		t.Errorf("TransferID: got %d, want %d", got.TransferID, orig.TransferID)
	}
	if got.Error != orig.Error {
		t.Errorf("Error: got %d, want %d", got.Error, orig.Error)
	}
}

func TestFileDownReq_Roundtrip(t *testing.T) {
	orig := FileDownReq{
		TransferID:  99887766,
		FilePathLen: 10,
		FilePath:    "/tmp/exfil",
		FilenameLen: 8,
		Filename:    "dump.tar",
	}
	data := orig.MarshalBinary()
	var got FileDownReq
	if err := got.UnmarshalBinary(data); err != nil {
		t.Fatalf("UnmarshalBinary failed: %v", err)
	}
	if got.TransferID != orig.TransferID {
		t.Errorf("TransferID: got %d, want %d", got.TransferID, orig.TransferID)
	}
	if got.FilePathLen != orig.FilePathLen {
		t.Errorf("FilePathLen: got %d, want %d", got.FilePathLen, orig.FilePathLen)
	}
	if got.FilePath != orig.FilePath {
		t.Errorf("FilePath: got %q, want %q", got.FilePath, orig.FilePath)
	}
	if got.FilenameLen != orig.FilenameLen {
		t.Errorf("FilenameLen: got %d, want %d", got.FilenameLen, orig.FilenameLen)
	}
	if got.Filename != orig.Filename {
		t.Errorf("Filename: got %q, want %q", got.Filename, orig.Filename)
	}
}

func TestFileDownRes_Roundtrip(t *testing.T) {
	orig := FileDownRes{
		TransferID: 55443322,
		OK:         1,
	}
	data := orig.MarshalBinary()
	var got FileDownRes
	if err := got.UnmarshalBinary(data); err != nil {
		t.Fatalf("UnmarshalBinary failed: %v", err)
	}
	if got.TransferID != orig.TransferID {
		t.Errorf("TransferID: got %d, want %d", got.TransferID, orig.TransferID)
	}
	if got.OK != orig.OK {
		t.Errorf("OK: got %d, want %d", got.OK, orig.OK)
	}
}

func TestSocksStart_Roundtrip(t *testing.T) {
	orig := SocksStart{
		UsernameLen: 8,
		Username:    "proxyusr",
		PasswordLen: 10,
		Password:    "s0cks!pass",
	}
	data := orig.MarshalBinary()
	var got SocksStart
	if err := got.UnmarshalBinary(data); err != nil {
		t.Fatalf("UnmarshalBinary failed: %v", err)
	}
	if got.UsernameLen != orig.UsernameLen {
		t.Errorf("UsernameLen: got %d, want %d", got.UsernameLen, orig.UsernameLen)
	}
	if got.Username != orig.Username {
		t.Errorf("Username: got %q, want %q", got.Username, orig.Username)
	}
	if got.PasswordLen != orig.PasswordLen {
		t.Errorf("PasswordLen: got %d, want %d", got.PasswordLen, orig.PasswordLen)
	}
	if got.Password != orig.Password {
		t.Errorf("Password: got %q, want %q", got.Password, orig.Password)
	}
}

func TestSocksUDPData_Roundtrip(t *testing.T) {
	payload := []byte{0x05, 0x00, 0x00, 0x01, 0x7F, 0x00, 0x00, 0x01, 0x04, 0x38, 0x48, 0x65, 0x6C, 0x6C, 0x6F}
	orig := SocksUDPData{
		Seq:     7777,
		DataLen: uint64(len(payload)),
		Data:    payload,
	}
	data := orig.MarshalBinary()
	var got SocksUDPData
	if err := got.UnmarshalBinary(data); err != nil {
		t.Fatalf("UnmarshalBinary failed: %v", err)
	}
	if got.Seq != orig.Seq {
		t.Errorf("Seq: got %d, want %d", got.Seq, orig.Seq)
	}
	if got.DataLen != orig.DataLen {
		t.Errorf("DataLen: got %d, want %d", got.DataLen, orig.DataLen)
	}
	if !bytes.Equal(got.Data, orig.Data) {
		t.Errorf("Data: got %x, want %x", got.Data, orig.Data)
	}
}

func TestUDPAssStart_Roundtrip(t *testing.T) {
	orig := UDPAssStart{
		Seq:           4096,
		SourceAddrLen: 18,
		SourceAddr:    "192.168.10.55:9090",
	}
	data := orig.MarshalBinary()
	var got UDPAssStart
	if err := got.UnmarshalBinary(data); err != nil {
		t.Fatalf("UnmarshalBinary failed: %v", err)
	}
	if got.Seq != orig.Seq {
		t.Errorf("Seq: got %d, want %d", got.Seq, orig.Seq)
	}
	if got.SourceAddrLen != orig.SourceAddrLen {
		t.Errorf("SourceAddrLen: got %d, want %d", got.SourceAddrLen, orig.SourceAddrLen)
	}
	if got.SourceAddr != orig.SourceAddr {
		t.Errorf("SourceAddr: got %q, want %q", got.SourceAddr, orig.SourceAddr)
	}
}

func TestUDPAssRes_Roundtrip(t *testing.T) {
	orig := UDPAssRes{
		Seq:     8192,
		OK:      1,
		AddrLen: 14,
		Addr:    "10.0.0.1:11223",
	}
	data := orig.MarshalBinary()
	var got UDPAssRes
	if err := got.UnmarshalBinary(data); err != nil {
		t.Fatalf("UnmarshalBinary failed: %v", err)
	}
	if got.Seq != orig.Seq {
		t.Errorf("Seq: got %d, want %d", got.Seq, orig.Seq)
	}
	if got.OK != orig.OK {
		t.Errorf("OK: got %d, want %d", got.OK, orig.OK)
	}
	if got.AddrLen != orig.AddrLen {
		t.Errorf("AddrLen: got %d, want %d", got.AddrLen, orig.AddrLen)
	}
	if got.Addr != orig.Addr {
		t.Errorf("Addr: got %q, want %q", got.Addr, orig.Addr)
	}
}

func TestSocksTCPFin_Roundtrip(t *testing.T) {
	orig := SocksTCPFin{
		Seq: 65535,
	}
	data := orig.MarshalBinary()
	var got SocksTCPFin
	if err := got.UnmarshalBinary(data); err != nil {
		t.Fatalf("UnmarshalBinary failed: %v", err)
	}
	if got.Seq != orig.Seq {
		t.Errorf("Seq: got %d, want %d", got.Seq, orig.Seq)
	}
}

func TestSocksReady_Roundtrip(t *testing.T) {
	orig := SocksReady{
		OK: 1,
	}
	data := orig.MarshalBinary()
	var got SocksReady
	if err := got.UnmarshalBinary(data); err != nil {
		t.Fatalf("UnmarshalBinary failed: %v", err)
	}
	if got.OK != orig.OK {
		t.Errorf("OK: got %d, want %d", got.OK, orig.OK)
	}
}

func TestForwardTest_Roundtrip(t *testing.T) {
	orig := ForwardTest{
		AddrLen: 15,
		Addr:    "172.16.0.1:8443",
	}
	data := orig.MarshalBinary()
	var got ForwardTest
	if err := got.UnmarshalBinary(data); err != nil {
		t.Fatalf("UnmarshalBinary failed: %v", err)
	}
	if got.AddrLen != orig.AddrLen {
		t.Errorf("AddrLen: got %d, want %d", got.AddrLen, orig.AddrLen)
	}
	if got.Addr != orig.Addr {
		t.Errorf("Addr: got %q, want %q", got.Addr, orig.Addr)
	}
}

func TestForwardStart_Roundtrip(t *testing.T) {
	orig := ForwardStart{
		Seq:     1024,
		AddrLen: 14,
		Addr:    "10.0.0.5:33060",
	}
	data := orig.MarshalBinary()
	var got ForwardStart
	if err := got.UnmarshalBinary(data); err != nil {
		t.Fatalf("UnmarshalBinary failed: %v", err)
	}
	if got.Seq != orig.Seq {
		t.Errorf("Seq: got %d, want %d", got.Seq, orig.Seq)
	}
	if got.AddrLen != orig.AddrLen {
		t.Errorf("AddrLen: got %d, want %d", got.AddrLen, orig.AddrLen)
	}
	if got.Addr != orig.Addr {
		t.Errorf("Addr: got %q, want %q", got.Addr, orig.Addr)
	}
}

func TestForwardReady_Roundtrip(t *testing.T) {
	orig := ForwardReady{
		OK: 1,
	}
	data := orig.MarshalBinary()
	var got ForwardReady
	if err := got.UnmarshalBinary(data); err != nil {
		t.Fatalf("UnmarshalBinary failed: %v", err)
	}
	if got.OK != orig.OK {
		t.Errorf("OK: got %d, want %d", got.OK, orig.OK)
	}
}

func TestForwardFin_Roundtrip(t *testing.T) {
	orig := ForwardFin{
		Seq: 2048,
	}
	data := orig.MarshalBinary()
	var got ForwardFin
	if err := got.UnmarshalBinary(data); err != nil {
		t.Fatalf("UnmarshalBinary failed: %v", err)
	}
	if got.Seq != orig.Seq {
		t.Errorf("Seq: got %d, want %d", got.Seq, orig.Seq)
	}
}

func TestBackwardTest_Roundtrip(t *testing.T) {
	orig := BackwardTest{
		LPortLen: 4,
		LPort:    "8080",
		RPortLen: 5,
		RPort:    "18080",
	}
	data := orig.MarshalBinary()
	var got BackwardTest
	if err := got.UnmarshalBinary(data); err != nil {
		t.Fatalf("UnmarshalBinary failed: %v", err)
	}
	if got.LPortLen != orig.LPortLen {
		t.Errorf("LPortLen: got %d, want %d", got.LPortLen, orig.LPortLen)
	}
	if got.LPort != orig.LPort {
		t.Errorf("LPort: got %q, want %q", got.LPort, orig.LPort)
	}
	if got.RPortLen != orig.RPortLen {
		t.Errorf("RPortLen: got %d, want %d", got.RPortLen, orig.RPortLen)
	}
	if got.RPort != orig.RPort {
		t.Errorf("RPort: got %q, want %q", got.RPort, orig.RPort)
	}
}

func TestBackwardStart_Roundtrip(t *testing.T) {
	orig := BackwardStart{
		UUIDLen:  10,
		UUID:     "BKWD000001",
		LPortLen: 4,
		LPort:    "3389",
		RPortLen: 5,
		RPort:    "13389",
	}
	data := orig.MarshalBinary()
	var got BackwardStart
	if err := got.UnmarshalBinary(data); err != nil {
		t.Fatalf("UnmarshalBinary failed: %v", err)
	}
	if got.UUIDLen != orig.UUIDLen {
		t.Errorf("UUIDLen: got %d, want %d", got.UUIDLen, orig.UUIDLen)
	}
	if got.UUID != orig.UUID {
		t.Errorf("UUID: got %q, want %q", got.UUID, orig.UUID)
	}
	if got.LPortLen != orig.LPortLen {
		t.Errorf("LPortLen: got %d, want %d", got.LPortLen, orig.LPortLen)
	}
	if got.LPort != orig.LPort {
		t.Errorf("LPort: got %q, want %q", got.LPort, orig.LPort)
	}
	if got.RPortLen != orig.RPortLen {
		t.Errorf("RPortLen: got %d, want %d", got.RPortLen, orig.RPortLen)
	}
	if got.RPort != orig.RPort {
		t.Errorf("RPort: got %q, want %q", got.RPort, orig.RPort)
	}
}

func TestBackwardReady_Roundtrip(t *testing.T) {
	orig := BackwardReady{
		OK: 1,
	}
	data := orig.MarshalBinary()
	var got BackwardReady
	if err := got.UnmarshalBinary(data); err != nil {
		t.Fatalf("UnmarshalBinary failed: %v", err)
	}
	if got.OK != orig.OK {
		t.Errorf("OK: got %d, want %d", got.OK, orig.OK)
	}
}

func TestBackwardSeq_Roundtrip(t *testing.T) {
	orig := BackwardSeq{
		Seq:      131072,
		RPortLen: 5,
		RPort:    "23456",
	}
	data := orig.MarshalBinary()
	var got BackwardSeq
	if err := got.UnmarshalBinary(data); err != nil {
		t.Fatalf("UnmarshalBinary failed: %v", err)
	}
	if got.Seq != orig.Seq {
		t.Errorf("Seq: got %d, want %d", got.Seq, orig.Seq)
	}
	if got.RPortLen != orig.RPortLen {
		t.Errorf("RPortLen: got %d, want %d", got.RPortLen, orig.RPortLen)
	}
	if got.RPort != orig.RPort {
		t.Errorf("RPort: got %q, want %q", got.RPort, orig.RPort)
	}
}

func TestBackWardFin_Roundtrip(t *testing.T) {
	orig := BackWardFin{
		Seq: 0x0102030405060708,
	}
	data := orig.MarshalBinary()
	var got BackWardFin
	if err := got.UnmarshalBinary(data); err != nil {
		t.Fatalf("UnmarshalBinary failed: %v", err)
	}
	if got.Seq != orig.Seq {
		t.Errorf("Seq: got %d, want %d", got.Seq, orig.Seq)
	}
}

func TestBackwardStop_Roundtrip(t *testing.T) {
	orig := BackwardStop{
		All:      1,
		RPortLen: 5,
		RPort:    "54321",
	}
	data := orig.MarshalBinary()
	var got BackwardStop
	if err := got.UnmarshalBinary(data); err != nil {
		t.Fatalf("UnmarshalBinary failed: %v", err)
	}
	if got.All != orig.All {
		t.Errorf("All: got %d, want %d", got.All, orig.All)
	}
	if got.RPortLen != orig.RPortLen {
		t.Errorf("RPortLen: got %d, want %d", got.RPortLen, orig.RPortLen)
	}
	if got.RPort != orig.RPort {
		t.Errorf("RPort: got %q, want %q", got.RPort, orig.RPort)
	}
}

func TestBackwardStopDone_Roundtrip(t *testing.T) {
	orig := BackwardStopDone{
		All:      1,
		UUIDLen:  10,
		UUID:     "STOP00DONE",
		RPortLen: 5,
		RPort:    "44556",
	}
	data := orig.MarshalBinary()
	var got BackwardStopDone
	if err := got.UnmarshalBinary(data); err != nil {
		t.Fatalf("UnmarshalBinary failed: %v", err)
	}
	if got.All != orig.All {
		t.Errorf("All: got %d, want %d", got.All, orig.All)
	}
	if got.UUIDLen != orig.UUIDLen {
		t.Errorf("UUIDLen: got %d, want %d", got.UUIDLen, orig.UUIDLen)
	}
	if got.UUID != orig.UUID {
		t.Errorf("UUID: got %q, want %q", got.UUID, orig.UUID)
	}
	if got.RPortLen != orig.RPortLen {
		t.Errorf("RPortLen: got %d, want %d", got.RPortLen, orig.RPortLen)
	}
	if got.RPort != orig.RPort {
		t.Errorf("RPort: got %q, want %q", got.RPort, orig.RPort)
	}
}

func TestConnectStart_Roundtrip(t *testing.T) {
	orig := ConnectStart{
		AddrLen: 18,
		Addr:    "192.168.100.1:4443",
	}
	data := orig.MarshalBinary()
	var got ConnectStart
	if err := got.UnmarshalBinary(data); err != nil {
		t.Fatalf("UnmarshalBinary failed: %v", err)
	}
	if got.AddrLen != orig.AddrLen {
		t.Errorf("AddrLen: got %d, want %d", got.AddrLen, orig.AddrLen)
	}
	if got.Addr != orig.Addr {
		t.Errorf("Addr: got %q, want %q", got.Addr, orig.Addr)
	}
}

func TestConnectDone_Roundtrip(t *testing.T) {
	orig := ConnectDone{
		OK: 1,
	}
	data := orig.MarshalBinary()
	var got ConnectDone
	if err := got.UnmarshalBinary(data); err != nil {
		t.Fatalf("UnmarshalBinary failed: %v", err)
	}
	if got.OK != orig.OK {
		t.Errorf("OK: got %d, want %d", got.OK, orig.OK)
	}
}

func TestNodeOffline_Roundtrip(t *testing.T) {
	orig := NodeOffline{
		UUIDLen: 10,
		UUID:    "OFFLINE001",
	}
	data := orig.MarshalBinary()
	var got NodeOffline
	if err := got.UnmarshalBinary(data); err != nil {
		t.Fatalf("UnmarshalBinary failed: %v", err)
	}
	if got.UUIDLen != orig.UUIDLen {
		t.Errorf("UUIDLen: got %d, want %d", got.UUIDLen, orig.UUIDLen)
	}
	if got.UUID != orig.UUID {
		t.Errorf("UUID: got %q, want %q", got.UUID, orig.UUID)
	}
}

func TestNodeReonline_Roundtrip(t *testing.T) {
	orig := NodeReonline{
		ParentUUIDLen: 10,
		ParentUUID:    "PARENTNODE",
		UUIDLen:       10,
		UUID:          "CHILDNODE1",
		IPLen:         11,
		IP:            "10.10.10.55",
	}
	data := orig.MarshalBinary()
	var got NodeReonline
	if err := got.UnmarshalBinary(data); err != nil {
		t.Fatalf("UnmarshalBinary failed: %v", err)
	}
	if got.ParentUUIDLen != orig.ParentUUIDLen {
		t.Errorf("ParentUUIDLen: got %d, want %d", got.ParentUUIDLen, orig.ParentUUIDLen)
	}
	if got.ParentUUID != orig.ParentUUID {
		t.Errorf("ParentUUID: got %q, want %q", got.ParentUUID, orig.ParentUUID)
	}
	if got.UUIDLen != orig.UUIDLen {
		t.Errorf("UUIDLen: got %d, want %d", got.UUIDLen, orig.UUIDLen)
	}
	if got.UUID != orig.UUID {
		t.Errorf("UUID: got %q, want %q", got.UUID, orig.UUID)
	}
	if got.IPLen != orig.IPLen {
		t.Errorf("IPLen: got %d, want %d", got.IPLen, orig.IPLen)
	}
	if got.IP != orig.IP {
		t.Errorf("IP: got %q, want %q", got.IP, orig.IP)
	}
}

func TestUpstreamOffline_Roundtrip(t *testing.T) {
	orig := UpstreamOffline{
		OK: 1,
	}
	data := orig.MarshalBinary()
	var got UpstreamOffline
	if err := got.UnmarshalBinary(data); err != nil {
		t.Fatalf("UnmarshalBinary failed: %v", err)
	}
	if got.OK != orig.OK {
		t.Errorf("OK: got %d, want %d", got.OK, orig.OK)
	}
}

func TestUpstreamReonline_Roundtrip(t *testing.T) {
	orig := UpstreamReonline{
		OK: 1,
	}
	data := orig.MarshalBinary()
	var got UpstreamReonline
	if err := got.UnmarshalBinary(data); err != nil {
		t.Fatalf("UnmarshalBinary failed: %v", err)
	}
	if got.OK != orig.OK {
		t.Errorf("OK: got %d, want %d", got.OK, orig.OK)
	}
}

func TestShutdown_Roundtrip(t *testing.T) {
	orig := Shutdown{
		OK: 1,
	}
	data := orig.MarshalBinary()
	var got Shutdown
	if err := got.UnmarshalBinary(data); err != nil {
		t.Fatalf("UnmarshalBinary failed: %v", err)
	}
	if got.OK != orig.OK {
		t.Errorf("OK: got %d, want %d", got.OK, orig.OK)
	}
}

func TestHeartbeatMsg_Roundtrip(t *testing.T) {
	orig := HeartbeatMsg{
		Ping: 0xBEEF,
	}
	data := orig.MarshalBinary()
	var got HeartbeatMsg
	if err := got.UnmarshalBinary(data); err != nil {
		t.Fatalf("UnmarshalBinary failed: %v", err)
	}
	if got.Ping != orig.Ping {
		t.Errorf("Ping: got %d, want %d", got.Ping, orig.Ping)
	}
}

func TestTransportSwitchReq_Roundtrip(t *testing.T) {
	orig := TransportSwitchReq{
		Method:  3,
		AddrLen: 21,
		Addr:    "transport.example.com",
	}
	data := orig.MarshalBinary()
	var got TransportSwitchReq
	if err := got.UnmarshalBinary(data); err != nil {
		t.Fatalf("UnmarshalBinary failed: %v", err)
	}
	if got.Method != orig.Method {
		t.Errorf("Method: got %d, want %d", got.Method, orig.Method)
	}
	if got.AddrLen != orig.AddrLen {
		t.Errorf("AddrLen: got %d, want %d", got.AddrLen, orig.AddrLen)
	}
	if got.Addr != orig.Addr {
		t.Errorf("Addr: got %q, want %q", got.Addr, orig.Addr)
	}
}

func TestTransportSwitchRes_Roundtrip(t *testing.T) {
	orig := TransportSwitchRes{
		OK:      1,
		AddrLen: 15,
		Addr:    "10.0.0.1:443/ws",
	}
	data := orig.MarshalBinary()
	var got TransportSwitchRes
	if err := got.UnmarshalBinary(data); err != nil {
		t.Fatalf("UnmarshalBinary failed: %v", err)
	}
	if got.OK != orig.OK {
		t.Errorf("OK: got %d, want %d", got.OK, orig.OK)
	}
	if got.AddrLen != orig.AddrLen {
		t.Errorf("AddrLen: got %d, want %d", got.AddrLen, orig.AddrLen)
	}
	if got.Addr != orig.Addr {
		t.Errorf("Addr: got %q, want %q", got.Addr, orig.Addr)
	}
}

func TestTransportSwitchDone_Roundtrip(t *testing.T) {
	orig := TransportSwitchDone{
		OK: 1,
	}
	data := orig.MarshalBinary()
	var got TransportSwitchDone
	if err := got.UnmarshalBinary(data); err != nil {
		t.Fatalf("UnmarshalBinary failed: %v", err)
	}
	if got.OK != orig.OK {
		t.Errorf("OK: got %d, want %d", got.OK, orig.OK)
	}
}
