package utils

import (
	"net"
	"testing"
)

func TestGenerateUUID(t *testing.T) {
	id := GenerateUUID()
	if len(id) != 10 {
		t.Fatalf("expected length 10, got %d", len(id))
	}
	id2 := GenerateUUID()
	if id == id2 {
		t.Fatal("two calls returned the same UUID")
	}
}

func TestGetStringMd5(t *testing.T) {
	// MD5("hello") = 5d41402abc4b2a76b9719d911017c592
	got := GetStringMd5("hello")
	want := "5d41402abc4b2a76b9719d911017c592"
	if got != want {
		t.Fatalf("GetStringMd5(\"hello\") = %s, want %s", got, want)
	}
}

func TestGetStringSha256(t *testing.T) {
	// SHA256("hello") = 2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824
	got := GetStringSha256("hello")
	want := "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
	if got != want {
		t.Fatalf("GetStringSha256(\"hello\") = %s, want %s", got, want)
	}
}

func TestStringSliceReverse(t *testing.T) {
	// nil — should not panic
	StringSliceReverse(nil)

	// empty
	empty := []string{}
	StringSliceReverse(empty)
	if len(empty) != 0 {
		t.Fatal("empty slice should remain empty")
	}

	// odd length
	odd := []string{"a", "b", "c"}
	StringSliceReverse(odd)
	if odd[0] != "c" || odd[1] != "b" || odd[2] != "a" {
		t.Fatalf("odd reverse: got %v", odd)
	}

	// even length
	even := []string{"1", "2", "3", "4"}
	StringSliceReverse(even)
	if even[0] != "4" || even[1] != "3" || even[2] != "2" || even[3] != "1" {
		t.Fatalf("even reverse: got %v", even)
	}
}

func TestStr2Int(t *testing.T) {
	n, err := Str2Int("42")
	if err != nil || n != 42 {
		t.Fatalf("Str2Int(\"42\") = %d, %v", n, err)
	}

	_, err = Str2Int("abc")
	if err == nil {
		t.Fatal("expected error for non-numeric string")
	}
}

func TestInt2Str(t *testing.T) {
	s := Int2Str(42)
	if s != "42" {
		t.Fatalf("Int2Str(42) = %s", s)
	}
}

func TestStr2IntInt2StrRoundtrip(t *testing.T) {
	s := Int2Str(12345)
	n, err := Str2Int(s)
	if err != nil || n != 12345 {
		t.Fatalf("roundtrip failed: %d, %v", n, err)
	}
}

func TestCheckSystem(t *testing.T) {
	// Test environment is Windows, so we expect 0x01.
	got := CheckSystem()
	if got != 0x01 {
		t.Fatalf("CheckSystem() = 0x%02x, want 0x01 on Windows", got)
	}
}

func TestGetSystemInfo(t *testing.T) {
	hostname, username := GetSystemInfo()
	if hostname == "" {
		t.Fatal("hostname is empty")
	}
	if username == "" {
		t.Fatal("username is empty")
	}
}

func TestCheckIPPort(t *testing.T) {
	// port only
	normal, reuse, err := CheckIPPort("8080")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if normal != "127.0.0.1:8080" {
		t.Fatalf("normalAddr = %s, want 127.0.0.1:8080", normal)
	}
	if reuse != "0.0.0.0:8080" {
		t.Fatalf("reuseAddr = %s, want 0.0.0.0:8080", reuse)
	}

	// ip:port
	normal, reuse, err = CheckIPPort("127.0.0.1:8080")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if normal != "127.0.0.1:8080" {
		t.Fatalf("normalAddr = %s", normal)
	}
	if reuse != "0.0.0.0:8080" {
		t.Fatalf("reuseAddr = %s", reuse)
	}

	// invalid: not a number
	_, _, err = CheckIPPort("abc")
	if err == nil {
		t.Fatal("expected error for invalid port")
	}

	// port out of range
	_, _, err = CheckIPPort("0")
	if err == nil {
		t.Fatal("expected error for port 0")
	}
	_, _, err = CheckIPPort("99999")
	if err == nil {
		t.Fatal("expected error for port > 65535")
	}

	// too many colons
	_, _, err = CheckIPPort("1:2:3")
	if err == nil {
		t.Fatal("expected error for too many colons")
	}
}

func TestCheckIfIP4(t *testing.T) {
	if !CheckIfIP4("1.2.3.4") {
		t.Fatal("expected true for IPv4")
	}
	if CheckIfIP4("::1") {
		t.Fatal("expected false for IPv6")
	}
	if CheckIfIP4("") {
		t.Fatal("expected false for empty")
	}
}

func TestCheckRange(t *testing.T) {
	nodes := []int{5, 3, 1, 4, 2}
	CheckRange(nodes)
	for i := 1; i < len(nodes); i++ {
		if nodes[i] < nodes[i-1] {
			t.Fatalf("not sorted: %v", nodes)
		}
	}

	// already sorted
	sorted := []int{1, 2, 3}
	CheckRange(sorted)
	if sorted[0] != 1 || sorted[1] != 2 || sorted[2] != 3 {
		t.Fatalf("already sorted changed: %v", sorted)
	}
}

func TestGetDigitLen(t *testing.T) {
	tests := []struct {
		num  int
		want int
	}{
		{0, 1},
		{1, 1},
		{10, 2},
		{100, 3},
		{9999, 4},
	}
	for _, tt := range tests {
		got := GetDigitLen(tt.num)
		if got != tt.want {
			t.Errorf("GetDigitLen(%d) = %d, want %d", tt.num, got, tt.want)
		}
	}
}

func TestGetRandomString(t *testing.T) {
	for _, l := range []int{0, 1, 16, 32} {
		s := GetRandomString(l)
		if len(s) != l {
			t.Errorf("GetRandomString(%d) returned len %d", l, len(s))
		}
	}
}

func TestGetRandomInt(t *testing.T) {
	max := 100
	for i := 0; i < 50; i++ {
		v := GetRandomInt(max)
		if v < 0 || v >= max {
			t.Fatalf("GetRandomInt(%d) = %d, out of range", max, v)
		}
	}
}

func TestParseFileCommand(t *testing.T) {
	// exactly 2 args
	a, b, err := ParseFileCommand([]string{"upload", "/tmp/file"})
	if err != nil || a != "upload" || b != "/tmp/file" {
		t.Fatalf("2 args: %s, %s, %v", a, b, err)
	}

	// quoted args with spaces
	a, b, err = ParseFileCommand([]string{"upload", `"C:\my`, `folder\file.txt"`})
	if err != nil {
		t.Fatalf("quoted args error: %v", err)
	}
	if a != "upload" || b != `C:\my folder\file.txt` {
		t.Fatalf("quoted args: a=%s b=%s", a, b)
	}

	// not enough arguments
	_, _, err = ParseFileCommand([]string{"upload"})
	if err == nil {
		t.Fatal("expected error for 1 arg")
	}

	// invalid: 3 args without quotes
	_, _, err = ParseFileCommand([]string{"a", "b", "c"})
	if err == nil {
		t.Fatal("expected error for 3 unquoted args")
	}
}

func TestConvertStr2GBKAndBack(t *testing.T) {
	// ASCII roundtrip
	ascii := "hello"
	gbk := ConvertStr2GBK(ascii)
	back := ConvertGBK2Str(gbk)
	if back != ascii {
		t.Fatalf("ASCII roundtrip failed: got %s", back)
	}

	// Chinese characters roundtrip
	chinese := "你好世界"
	gbk = ConvertStr2GBK(chinese)
	back = ConvertGBK2Str(gbk)
	if back != chinese {
		t.Fatalf("Chinese roundtrip failed: got %s", back)
	}
}

func TestWriteFull(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	data := []byte("hello world, this is a test payload")

	errCh := make(chan error, 1)
	go func() {
		errCh <- WriteFull(client, data)
	}()

	buf := make([]byte, len(data))
	n := 0
	for n < len(data) {
		rn, err := server.Read(buf[n:])
		if err != nil {
			t.Fatalf("Read error: %v", err)
		}
		n += rn
	}

	if err := <-errCh; err != nil {
		t.Fatalf("WriteFull error: %v", err)
	}
	if string(buf) != string(data) {
		t.Fatalf("data mismatch: got %s", string(buf))
	}
}

func TestSafeSend(t *testing.T) {
	// normal send
	ch := make(chan []byte, 1)
	ok := SafeSend(ch, []byte("test"))
	if !ok {
		t.Fatal("SafeSend returned false on open channel")
	}
	got := <-ch
	if string(got) != "test" {
		t.Fatalf("received %s", string(got))
	}

	// send to closed channel should not panic, returns false
	close(ch)
	ok = SafeSend(ch, []byte("test"))
	// SafeSend recovers from the panic but the deferred recover
	// means the named return is the zero value (false).
	if ok {
		t.Fatal("SafeSend returned true on closed channel")
	}
}
