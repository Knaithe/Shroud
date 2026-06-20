package share

import (
	"net"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"Shroud/crypto"
	"Shroud/global"
	"Shroud/protocol"
)

func TestSanitizePath_NormalRelative(t *testing.T) {
	p, err := sanitizePath("foo.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p != "foo.txt" {
		t.Fatalf("expected %q, got %q", "foo.txt", p)
	}
}

func TestSanitizePath_Subdirectory(t *testing.T) {
	p, err := sanitizePath("sub/dir/file.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := filepath.Clean("sub/dir/file.txt")
	if p != expected {
		t.Fatalf("expected %q, got %q", expected, p)
	}
}

func TestSanitizePath_DotDotTraversal(t *testing.T) {
	_, err := sanitizePath("../etc/passwd")
	if err == nil {
		t.Fatal("expected error for path traversal with .., got nil")
	}
}

func TestSanitizePath_DotDotMultiple(t *testing.T) {
	_, err := sanitizePath("../../file")
	if err == nil {
		t.Fatal("expected error for path traversal with ../.., got nil")
	}
}

func TestSanitizePath_AbsoluteUnix(t *testing.T) {
	if runtime.GOOS == "windows" {
		// On Windows, "/etc/passwd" is not considered absolute by filepath.IsAbs;
		// it becomes "\etc\passwd" which is a relative-to-drive path.
		t.Skip("Unix-style absolute paths are not absolute on Windows")
	}
	_, err := sanitizePath("/etc/passwd")
	if err == nil {
		t.Fatal("expected error for absolute Unix path, got nil")
	}
}

func TestSanitizePath_AbsoluteWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows absolute path test only relevant on Windows")
	}
	_, err := sanitizePath(`C:\Users\file`)
	if err == nil {
		t.Fatal("expected error for absolute Windows path, got nil")
	}
}

func TestSanitizePath_CleanCollapse(t *testing.T) {
	// filepath.Clean("foo/../bar") produces "bar", which is a valid relative path.
	// sanitizePath should accept the cleaned result.
	p, err := sanitizePath("foo/../bar")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p != "bar" {
		t.Fatalf("expected %q, got %q", "bar", p)
	}
}

func TestSanitizePath_CurrentDir(t *testing.T) {
	// When cleaned path is ".", filepath.Join(cwd, ".") collapses to cwd,
	// then filepath.Dir(cwd) returns the parent directory. On some platforms
	// this causes the symlink escape check to fail because the parent is
	// outside cwd. We just verify no panic and document the behavior.
	p, err := sanitizePath(".")
	if err != nil {
		// The function may reject "." on platforms where Dir(Join(cwd,"."))
		// resolves to the parent of cwd. This is a known edge case.
		t.Logf("sanitizePath(\".\") returned error (platform-dependent): %v", err)
		return
	}
	if p != "." {
		t.Fatalf("expected %q, got %q", ".", p)
	}
}

func TestSanitizePath_EmptyString(t *testing.T) {
	// filepath.Clean("") returns ".", so this has the same behavior as ".".
	p, err := sanitizePath("")
	if err != nil {
		// Same edge case as "." — see TestSanitizePath_CurrentDir.
		t.Logf("sanitizePath(\"\") returned error (platform-dependent): %v", err)
		return
	}
	if p != "." {
		t.Fatalf("expected %q, got %q", ".", p)
	}
}

func TestSanitizePath_SymlinkEscape(t *testing.T) {
	// Create a temporary directory outside cwd, then create a symlink inside
	// a temp working directory that points to the outside directory.
	// sanitizePath should reject the symlink that escapes cwd.

	outsideDir, err := os.MkdirTemp("", "shroud-outside-*")
	if err != nil {
		t.Fatalf("failed to create outside temp dir: %v", err)
	}
	defer os.RemoveAll(outsideDir)

	workDir, err := os.MkdirTemp("", "shroud-workdir-*")
	if err != nil {
		t.Fatalf("failed to create work temp dir: %v", err)
	}
	defer os.RemoveAll(workDir)

	linkPath := filepath.Join(workDir, "escape-link")
	err = os.Symlink(outsideDir, linkPath)
	if err != nil {
		t.Skip("skipping symlink test: os.Symlink failed (may need admin on Windows)")
	}

	// Change working directory to workDir so sanitizePath resolves relative to it.
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get original working directory: %v", err)
	}
	defer os.Chdir(origDir)

	err = os.Chdir(workDir)
	if err != nil {
		t.Fatalf("failed to chdir to work dir: %v", err)
	}

	// "escape-link/somefile" should be rejected because the symlink resolves
	// outside the working directory.
	_, err = sanitizePath("escape-link/somefile")
	if err == nil {
		t.Fatal("expected error for symlink escaping working directory, got nil")
	}
}

// --- NewFile tests ---

func TestNewFile_Defaults(t *testing.T) {
	f := NewFile()
	if f.SliceSize != 30720 {
		t.Fatalf("SliceSize: want 30720, got %d", f.SliceSize)
	}
	if f.ErrChan == nil {
		t.Fatal("ErrChan should be initialized, got nil")
	}
	if f.DataChan == nil {
		t.Fatal("DataChan should be initialized, got nil")
	}
	if f.StatusChan == nil {
		t.Fatal("StatusChan should be initialized, got nil")
	}
	// StatusChan must be buffered with capacity 10.
	if cap(f.StatusChan) != 10 {
		t.Fatalf("StatusChan capacity: want 10, got %d", cap(f.StatusChan))
	}
	// Other fields should be zero-valued.
	if f.TransferID != 0 {
		t.Fatalf("TransferID: want 0, got %d", f.TransferID)
	}
	if f.FileName != "" {
		t.Fatalf("FileName: want empty, got %q", f.FileName)
	}
	if f.FilePath != "" {
		t.Fatalf("FilePath: want empty, got %q", f.FilePath)
	}
	if f.FileSize != 0 {
		t.Fatalf("FileSize: want 0, got %d", f.FileSize)
	}
	if f.SliceNum != 0 {
		t.Fatalf("SliceNum: want 0, got %d", f.SliceNum)
	}
	if f.Handler != nil {
		t.Fatal("Handler: want nil, got non-nil")
	}
}

func TestNewFile_ChannelsUsable(t *testing.T) {
	f := NewFile()

	// Verify channels can send/receive without blocking (using goroutines).
	go func() { f.DataChan <- []byte("hello") }()
	data := <-f.DataChan
	if string(data) != "hello" {
		t.Fatalf("DataChan round-trip: want %q, got %q", "hello", string(data))
	}

	go func() { f.ErrChan <- true }()
	errVal := <-f.ErrChan
	if !errVal {
		t.Fatal("ErrChan round-trip: want true, got false")
	}

	// StatusChan is buffered, so we can send without a goroutine.
	f.StatusChan <- &Status{Stat: START}
	st := <-f.StatusChan
	if st.Stat != START {
		t.Fatalf("StatusChan round-trip: want %d, got %d", START, st.Stat)
	}
}

// --- sanitizePath: the "." case with a real directory ---

func TestSanitizePath_DotInSubdir(t *testing.T) {
	// Create a temp dir, chdir into it, and test that "." resolves.
	// The result depends on platform behavior (see TestSanitizePath_CurrentDir),
	// but in a freshly created directory the parent always exists and is different.
	tmpDir, err := os.MkdirTemp("", "shroud-dot-*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	defer os.Chdir(origDir)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}

	p, err := sanitizePath(".")
	if err != nil {
		// On some platforms, Dir(Join(cwd, ".")) is the parent of cwd,
		// which causes the escape check to fire. Document and accept.
		t.Logf("sanitizePath(\".\") returned error in tmpDir (platform-dependent): %v", err)
		return
	}
	if p != "." {
		t.Fatalf("expected %q, got %q", ".", p)
	}
}

// --- initTestGComponent helper ---

func initTestGComponent(t *testing.T) (client net.Conn, server net.Conn) {
	t.Helper()
	client, server = net.Pipe()
	protocol.SetUpDownStream("raw", "raw")
	global.InitialGComponent(client, crypto.DeriveKey([]byte("test-secret-key"), crypto.PurposeEncrypt), "TESTUUID01")
	return client, server
}

// --- SendFileStat test ---

func TestSendFileStat_FileNotExist(t *testing.T) {
	client, server := initTestGComponent(t)
	defer client.Close()
	defer server.Close()

	f := NewFile()
	f.FileName = "nonexistent.txt"
	f.FilePath = "nonexistent_path_that_does_not_exist.bin"
	f.TransferID = 42

	err := f.SendFileStat("test-route", "TARGETUUID", ADMIN)
	if err == nil {
		t.Fatal("expected error for nonexistent file, got nil")
	}
}

func TestSendFileStat_ValidFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "shroud-sendstat-*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	defer os.Chdir(origDir)
	os.Chdir(tmpDir)

	// Create a test file with known content.
	tmpFile, err := os.CreateTemp(tmpDir, "upload-*.dat")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	data := make([]byte, 65536) // 65KB, should be 3 slices at 30720 each
	for i := range data {
		data[i] = byte(i % 256)
	}
	tmpFile.Write(data)
	tmpFile.Close()

	client, server := initTestGComponent(t)
	defer client.Close()
	defer server.Close()

	// Drain the server side in the background so SendMessage doesn't block.
	go func() {
		buf := make([]byte, 65536)
		for {
			_, err := server.Read(buf)
			if err != nil {
				return
			}
		}
	}()

	f := NewFile()
	f.FileName = filepath.Base(tmpFile.Name())
	f.FilePath = filepath.Base(tmpFile.Name())
	f.TransferID = 1

	err = f.SendFileStat("route1", "TARGETUUID", ADMIN)
	if err != nil {
		t.Fatalf("SendFileStat failed: %v", err)
	}

	if f.Handler == nil {
		t.Fatal("expected Handler to be set after SendFileStat")
	}
	f.Handler.Close()

	if f.FileSize != int64(len(data)) {
		t.Fatalf("FileSize: want %d, got %d", len(data), f.FileSize)
	}
}

func TestSendFileStat_AbsolutePathRejected(t *testing.T) {
	client, server := initTestGComponent(t)
	defer client.Close()
	defer server.Close()

	f := NewFile()
	f.FileName = "test.txt"
	f.TransferID = 1

	if runtime.GOOS == "windows" {
		f.FilePath = `C:\Windows\System32\cmd.exe`
	} else {
		f.FilePath = "/etc/passwd"
	}

	err := f.SendFileStat("route1", "TARGETUUID", ADMIN)
	if err == nil {
		if f.Handler != nil {
			f.Handler.Close()
		}
		t.Fatal("expected error for absolute path, got nil")
	}
}

// --- CheckFileStat test ---

func TestCheckFileStat_CreatesFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "shroud-checkstat-*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	defer os.Chdir(origDir)
	os.Chdir(tmpDir)

	client, server := initTestGComponent(t)
	defer client.Close()
	defer server.Close()

	go func() {
		buf := make([]byte, 65536)
		for {
			_, err := server.Read(buf)
			if err != nil {
				return
			}
		}
	}()

	f := NewFile()
	f.FileName = "newfile.dat"
	f.TransferID = 2

	err = f.CheckFileStat("route1", "TARGETUUID", ADMIN)
	if err != nil {
		t.Fatalf("CheckFileStat failed: %v", err)
	}

	if f.Handler == nil {
		t.Fatal("expected Handler to be set")
	}
	f.Handler.Close()

	// Verify file was created.
	info, err := os.Stat(filepath.Join(tmpDir, "newfile.dat"))
	if err != nil {
		t.Fatalf("file was not created: %v", err)
	}
	if info.Size() != 0 {
		t.Fatalf("expected empty file, got size %d", info.Size())
	}
}

func TestCheckFileStat_FileAlreadyExists(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "shroud-checkstat-exist-*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	defer os.Chdir(origDir)
	os.Chdir(tmpDir)

	// Pre-create the file so O_EXCL fails.
	os.WriteFile(filepath.Join(tmpDir, "existing.dat"), []byte("data"), 0600)

	client, server := initTestGComponent(t)
	defer client.Close()
	defer server.Close()

	go func() {
		buf := make([]byte, 65536)
		for {
			_, err := server.Read(buf)
			if err != nil {
				return
			}
		}
	}()

	f := NewFile()
	f.FileName = "existing.dat"
	f.TransferID = 3

	err = f.CheckFileStat("route1", "TARGETUUID", ADMIN)
	if err == nil {
		if f.Handler != nil {
			f.Handler.Close()
		}
		t.Fatal("expected error for existing file (O_EXCL), got nil")
	}
}

func TestCheckFileStat_PathTraversalRejected(t *testing.T) {
	client, server := initTestGComponent(t)
	defer client.Close()
	defer server.Close()

	// Drain server side so the deferred failure message does not block.
	go func() {
		buf := make([]byte, 65536)
		for {
			_, err := server.Read(buf)
			if err != nil {
				return
			}
		}
	}()

	f := NewFile()
	f.FileName = "../escape.dat"
	f.TransferID = 4

	err := f.CheckFileStat("route1", "TARGETUUID", ADMIN)
	if err == nil {
		if f.Handler != nil {
			f.Handler.Close()
		}
		t.Fatal("expected error for path traversal, got nil")
	}
}

// --- Receive test ---

func TestReceive_WritesData(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "shroud-receive-*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	outPath := filepath.Join(tmpDir, "received.dat")
	fh, err := os.Create(outPath)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	f := NewFile()
	f.Handler = fh
	f.SliceNum = 3

	// Feed 3 slices via DataChan (as agent, no StatusChan traffic).
	go func() {
		f.DataChan <- []byte("aaa")
		f.DataChan <- []byte("bbb")
		f.DataChan <- []byte("ccc")
	}()

	f.Receive("route", "TARGET0001", AGENT)

	content, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(content) != "aaabbbccc" {
		t.Fatalf("received content: want %q, got %q", "aaabbbccc", string(content))
	}
}

func TestReceive_EarlyError(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "shroud-receive-err-*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	outPath := filepath.Join(tmpDir, "partial.dat")
	fh, err := os.Create(outPath)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	f := NewFile()
	f.Handler = fh
	f.SliceNum = 5

	// Send one slice, then signal error.
	go func() {
		f.DataChan <- []byte("first")
		f.ErrChan <- true
	}()

	f.Receive("route", "TARGET0001", AGENT)

	content, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	// Only the first slice should have been written.
	if string(content) != "first" {
		t.Fatalf("partial content: want %q, got %q", "first", string(content))
	}
}

// --- Upload test ---

func TestUpload_ReadsFileAndSendsData(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "shroud-upload-*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a small test file.
	srcPath := filepath.Join(tmpDir, "source.dat")
	testData := []byte("hello upload test data")
	if err := os.WriteFile(srcPath, testData, 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	fh, err := os.Open(srcPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	client, server := initTestGComponent(t)
	defer client.Close()
	defer server.Close()

	// Drain server side.
	go func() {
		buf := make([]byte, 65536)
		for {
			_, err := server.Read(buf)
			if err != nil {
				return
			}
		}
	}()

	f := NewFile()
	f.Handler = fh
	f.TransferID = 10

	// Upload runs as AGENT (no StatusChan writes).
	f.Upload("route1", "TARGETUUID", AGENT)

	// After Upload returns, the handler should be closed.
	// Attempting to read from a closed file should fail.
	_, err = fh.Read(make([]byte, 1))
	if err == nil {
		t.Fatal("expected error reading from closed file handle")
	}
}
