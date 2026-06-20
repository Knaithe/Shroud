package manager

import (
	"net"
	"sync"
	"testing"
)

// --- NewManager ---

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
	if m.ConsoleManager == nil {
		t.Error("ConsoleManager is nil")
	}
	if m.FileManager == nil {
		t.Error("FileManager is nil")
	}
	if m.SocksManager == nil {
		t.Error("SocksManager is nil")
	}
	if m.ForwardManager == nil {
		t.Error("ForwardManager is nil")
	}
	if m.BackwardManager == nil {
		t.Error("BackwardManager is nil")
	}
	if m.SSHManager == nil {
		t.Error("SSHManager is nil")
	}
	if m.SSHTunnelManager == nil {
		t.Error("SSHTunnelManager is nil")
	}
	if m.ShellManager == nil {
		t.Error("ShellManager is nil")
	}
	if m.InfoManager == nil {
		t.Error("InfoManager is nil")
	}
	if m.ListenManager == nil {
		t.Error("ListenManager is nil")
	}
	if m.ConnectManager == nil {
		t.Error("ConnectManager is nil")
	}
	if m.ChildrenManager == nil {
		t.Error("ChildrenManager is nil")
	}
	if m.TransportManager == nil {
		t.Error("TransportManager is nil")
	}
}

func TestManagerRun(t *testing.T) {
	m := NewManager()
	// Run is a no-op; just confirm it does not panic.
	m.Run()
}

// --- helpers ---

// newLocalListener creates a TCP listener on localhost for test use.
func newLocalListener(t *testing.T) net.Listener {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	return ln
}

// newConnPair returns a connected (client, server) pair of net.Conn.
func newConnPair(t *testing.T) (net.Conn, net.Conn) {
	t.Helper()
	ln := newLocalListener(t)
	defer ln.Close()

	var server net.Conn
	var serr error
	done := make(chan struct{})
	go func() {
		server, serr = ln.Accept()
		close(done)
	}()

	client, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	<-done
	if serr != nil {
		client.Close()
		t.Fatalf("accept: %v", serr)
	}
	return client, server
}

// ===========================================================================
// SocksManager tests
// ===========================================================================

func TestSocksManager_NewSocks(t *testing.T) {
	sm := newSocksManager()
	ln := newLocalListener(t)
	defer ln.Close()

	ok := sm.NewSocks("uuid1", "127.0.0.1", "1080", "user", "pass", ln)
	if !ok {
		t.Fatal("NewSocks should return true for a new uuid")
	}
	// duplicate uuid must be rejected
	ok = sm.NewSocks("uuid1", "0.0.0.0", "1081", "", "", ln)
	if ok {
		t.Fatal("NewSocks should return false for duplicate uuid")
	}
}

func TestSocksManager_GetSocksSeq(t *testing.T) {
	sm := newSocksManager()
	seq0 := sm.GetSocksSeq("uuid1")
	seq1 := sm.GetSocksSeq("uuid1")
	if seq0 != 0 || seq1 != 1 {
		t.Fatalf("expected seq 0,1 got %d,%d", seq0, seq1)
	}
}

func TestSocksManager_AddTCPSocket_And_GetDataChan(t *testing.T) {
	sm := newSocksManager()
	ln := newLocalListener(t)
	defer ln.Close()

	sm.NewSocks("uuid1", "127.0.0.1", "1080", "", "", ln)

	client, server := newConnPair(t)
	defer client.Close()
	defer server.Close()

	seq := sm.GetSocksSeq("uuid1")
	ok := sm.AddTCPSocket("uuid1", seq, client)
	if !ok {
		t.Fatal("AddTCPSocket should succeed")
	}

	// unknown uuid
	ok = sm.AddTCPSocket("no-such-uuid", seq, client)
	if ok {
		t.Fatal("AddTCPSocket should fail for unknown uuid")
	}

	ch, found := sm.GetTCPDataChan("uuid1", seq)
	if !found || ch == nil {
		t.Fatal("GetTCPDataChan should find the channel")
	}

	// unknown seq
	_, found = sm.GetTCPDataChan("uuid1", 999)
	if found {
		t.Fatal("GetTCPDataChan should return false for unknown seq")
	}

	// unknown uuid
	_, found = sm.GetTCPDataChan("bad-uuid", seq)
	if found {
		t.Fatal("GetTCPDataChan should return false for unknown uuid")
	}
}

func TestSocksManager_GetTCPDataChanBySeq(t *testing.T) {
	sm := newSocksManager()
	ln := newLocalListener(t)
	defer ln.Close()

	sm.NewSocks("uuid1", "127.0.0.1", "1080", "", "", ln)
	seq := sm.GetSocksSeq("uuid1")

	client, server := newConnPair(t)
	defer client.Close()
	defer server.Close()

	sm.AddTCPSocket("uuid1", seq, client)

	ch, ok := sm.GetTCPDataChanBySeq(seq)
	if !ok || ch == nil {
		t.Fatal("GetTCPDataChanBySeq should succeed")
	}

	_, ok = sm.GetTCPDataChanBySeq(999)
	if ok {
		t.Fatal("GetTCPDataChanBySeq should fail for unknown seq")
	}
}

func TestSocksManager_UDPFlow(t *testing.T) {
	sm := newSocksManager()
	ln := newLocalListener(t)
	defer ln.Close()

	sm.NewSocks("uuid1", "127.0.0.1", "1080", "", "", ln)
	seq := sm.GetSocksSeq("uuid1")

	client, server := newConnPair(t)
	defer client.Close()
	defer server.Close()

	sm.AddTCPSocket("uuid1", seq, client)

	// before UpdateUDP, udp channels should not exist — GetUDPDataChan would panic
	// UpdateUDP
	udpLn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("ListenPacket: %v", err)
	}
	defer udpLn.Close()

	ok := sm.UpdateUDP("uuid1", seq, udpLn.(*net.UDPConn))
	if !ok {
		t.Fatal("UpdateUDP should succeed")
	}

	ch, found := sm.GetUDPDataChan("uuid1", seq)
	if !found || ch == nil {
		t.Fatal("GetUDPDataChan should succeed after UpdateUDP")
	}

	ch2, found2 := sm.GetUDPDataChanBySeq(seq)
	if !found2 || ch2 == nil {
		t.Fatal("GetUDPDataChanBySeq should succeed")
	}

	_, found = sm.GetUDPDataChanBySeq(999)
	if found {
		t.Fatal("GetUDPDataChanBySeq should fail for unknown seq")
	}
}

func TestSocksManager_GetSocksInfo(t *testing.T) {
	sm := newSocksManager()
	ln := newLocalListener(t)
	defer ln.Close()

	sm.NewSocks("uuid1", "10.0.0.1", "1080", "admin", "secret", ln)

	info, ok := sm.GetSocksInfo("uuid1")
	if !ok {
		t.Fatal("GetSocksInfo should succeed")
	}
	if info.Addr != "10.0.0.1" || info.Port != "1080" || info.Username != "admin" || info.Password != "secret" {
		t.Fatalf("unexpected info: %+v", info)
	}

	_, ok = sm.GetSocksInfo("nonexistent")
	if ok {
		t.Fatal("GetSocksInfo should fail for unknown uuid")
	}
}

func TestSocksManager_CloseTCP(t *testing.T) {
	sm := newSocksManager()
	ln := newLocalListener(t)
	defer ln.Close()

	sm.NewSocks("uuid1", "127.0.0.1", "1080", "", "", ln)
	seq := sm.GetSocksSeq("uuid1")

	client, server := newConnPair(t)
	defer client.Close()
	defer server.Close()

	sm.AddTCPSocket("uuid1", seq, client)
	sm.CloseTCP(seq)

	_, found := sm.GetTCPDataChanBySeq(seq)
	if found {
		t.Fatal("seq should be removed after CloseTCP")
	}

	// closing unknown seq should not panic
	sm.CloseTCP(999)
}

func TestSocksManager_CloseSocks(t *testing.T) {
	sm := newSocksManager()
	ln := newLocalListener(t)

	sm.NewSocks("uuid1", "127.0.0.1", "1080", "", "", ln)
	seq := sm.GetSocksSeq("uuid1")

	client, server := newConnPair(t)
	defer client.Close()
	defer server.Close()

	sm.AddTCPSocket("uuid1", seq, client)
	sm.CloseSocks("uuid1")

	_, ok := sm.GetSocksInfo("uuid1")
	if ok {
		t.Fatal("socks entry should be removed after CloseSocks")
	}
}

func TestSocksManager_ForceShutdown(t *testing.T) {
	sm := newSocksManager()
	ln := newLocalListener(t)

	sm.NewSocks("uuid1", "127.0.0.1", "1080", "", "", ln)
	seq := sm.GetSocksSeq("uuid1")

	client, server := newConnPair(t)
	defer client.Close()
	defer server.Close()

	sm.AddTCPSocket("uuid1", seq, client)
	sm.ForceShutdown("uuid1")

	_, ok := sm.GetSocksInfo("uuid1")
	if ok {
		t.Fatal("socks entry should be removed after ForceShutdown")
	}

	// ForceShutdown on missing uuid should not panic
	sm.ForceShutdown("nonexistent")
}

func TestSocksManager_Concurrent(t *testing.T) {
	sm := newSocksManager()
	ln := newLocalListener(t)
	defer ln.Close()

	sm.NewSocks("uuid1", "127.0.0.1", "1080", "", "", ln)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = sm.GetSocksSeq("uuid1")
		}()
	}
	wg.Wait()
}

// ===========================================================================
// ForwardManager tests
// ===========================================================================

func TestForwardManager_NewForward(t *testing.T) {
	fm := newForwardManager()
	ln := newLocalListener(t)
	defer ln.Close()

	fm.NewForward("uuid1", "8080", "10.0.0.1:80", ln)
	// calling again for same uuid+port overwrites without error
	fm.NewForward("uuid1", "8080", "10.0.0.1:80", ln)
}

func TestForwardManager_GetNewSeq(t *testing.T) {
	fm := newForwardManager()
	s0 := fm.GetNewSeq("uuid1", "8080")
	s1 := fm.GetNewSeq("uuid1", "8081")
	if s0 != 0 || s1 != 1 {
		t.Fatalf("expected 0,1 got %d,%d", s0, s1)
	}
}

func TestForwardManager_AddConn_And_GetDataChan(t *testing.T) {
	fm := newForwardManager()
	ln := newLocalListener(t)
	defer ln.Close()

	fm.NewForward("uuid1", "8080", "10.0.0.1:80", ln)
	seq := fm.GetNewSeq("uuid1", "8080")

	ok := fm.AddConn("uuid1", "8080", seq)
	if !ok {
		t.Fatal("AddConn should succeed")
	}

	ch, found := fm.GetDataChan("uuid1", "8080", seq)
	if !found || ch == nil {
		t.Fatal("GetDataChan should succeed")
	}

	// unregistered seq
	ok = fm.AddConn("uuid1", "8080", 999)
	if ok {
		t.Fatal("AddConn should fail for unregistered seq")
	}

	_, found = fm.GetDataChan("uuid1", "8080", 999)
	if found {
		t.Fatal("GetDataChan should fail for unregistered seq")
	}
}

func TestForwardManager_GetDataChanBySeq(t *testing.T) {
	fm := newForwardManager()
	ln := newLocalListener(t)
	defer ln.Close()

	fm.NewForward("uuid1", "8080", "10.0.0.1:80", ln)
	seq := fm.GetNewSeq("uuid1", "8080")
	fm.AddConn("uuid1", "8080", seq)

	ch, ok := fm.GetDataChanBySeq(seq)
	if !ok || ch == nil {
		t.Fatal("GetDataChanBySeq should succeed")
	}

	_, ok = fm.GetDataChanBySeq(999)
	if ok {
		t.Fatal("GetDataChanBySeq should fail for unknown seq")
	}
}

func TestForwardManager_CloseTCP(t *testing.T) {
	fm := newForwardManager()
	ln := newLocalListener(t)
	defer ln.Close()

	fm.NewForward("uuid1", "8080", "10.0.0.1:80", ln)
	seq := fm.GetNewSeq("uuid1", "8080")
	fm.AddConn("uuid1", "8080", seq)

	fm.CloseTCP(seq)

	_, ok := fm.GetDataChanBySeq(seq)
	if ok {
		t.Fatal("data chan should be gone after CloseTCP")
	}

	// closing unknown seq should not panic
	fm.CloseTCP(999)
}

func TestForwardManager_CloseSingle(t *testing.T) {
	fm := newForwardManager()
	ln := newLocalListener(t)

	fm.NewForward("uuid1", "8080", "10.0.0.1:80", ln)
	seq := fm.GetNewSeq("uuid1", "8080")
	fm.AddConn("uuid1", "8080", seq)

	// GetForwardInfo populates forwardReadyDel
	infos, ok := fm.GetForwardInfo("uuid1")
	if !ok || len(infos) == 0 {
		t.Fatal("GetForwardInfo should succeed")
	}

	fm.CloseSingle("uuid1", infos[0].Seq)

	_, ok = fm.GetForwardInfo("uuid1")
	if ok {
		t.Fatal("forward should be removed after CloseSingle")
	}
}

func TestForwardManager_CloseSingleAll(t *testing.T) {
	fm := newForwardManager()
	ln1 := newLocalListener(t)
	ln2 := newLocalListener(t)

	fm.NewForward("uuid1", "8080", "10.0.0.1:80", ln1)
	fm.NewForward("uuid1", "8081", "10.0.0.1:81", ln2)
	fm.GetNewSeq("uuid1", "8080")
	fm.GetNewSeq("uuid1", "8081")

	fm.CloseSingleAll("uuid1")

	_, ok := fm.GetForwardInfo("uuid1")
	if ok {
		t.Fatal("all forwards should be removed")
	}
}

func TestForwardManager_ForceShutdown(t *testing.T) {
	fm := newForwardManager()
	ln := newLocalListener(t)

	fm.NewForward("uuid1", "8080", "10.0.0.1:80", ln)
	fm.GetNewSeq("uuid1", "8080")
	fm.ForceShutdown("uuid1")

	_, ok := fm.GetForwardInfo("uuid1")
	if ok {
		t.Fatal("forwards should be removed after ForceShutdown")
	}

	// missing uuid should not panic
	fm.ForceShutdown("nonexistent")
}

func TestForwardManager_GetForwardInfo(t *testing.T) {
	fm := newForwardManager()
	ln := newLocalListener(t)
	defer ln.Close()

	fm.NewForward("uuid1", "8080", "10.0.0.1:80", ln)

	infos, ok := fm.GetForwardInfo("uuid1")
	if !ok {
		t.Fatal("GetForwardInfo should succeed")
	}
	if len(infos) != 1 {
		t.Fatalf("expected 1 info, got %d", len(infos))
	}
	if infos[0].Raddr != "10.0.0.1:80" {
		t.Fatalf("unexpected Raddr: %s", infos[0].Raddr)
	}

	_, ok = fm.GetForwardInfo("nonexistent")
	if ok {
		t.Fatal("GetForwardInfo should fail for unknown uuid")
	}
}

func TestForwardManager_Concurrent(t *testing.T) {
	fm := newForwardManager()
	ln := newLocalListener(t)
	defer ln.Close()

	fm.NewForward("uuid1", "8080", "10.0.0.1:80", ln)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = fm.GetNewSeq("uuid1", "8080")
		}()
	}
	wg.Wait()
}

// ===========================================================================
// BackwardManager tests
// ===========================================================================

func TestBackwardManager_NewBackward(t *testing.T) {
	bm := newBackwardManager()
	bm.NewBackward("uuid1", "3000", "4000")

	// calling again overwrites
	bm.NewBackward("uuid1", "3001", "4000")
}

func TestBackwardManager_GetNewSeq(t *testing.T) {
	bm := newBackwardManager()
	s0 := bm.GetNewSeq("uuid1", "4000")
	s1 := bm.GetNewSeq("uuid1", "4000")
	if s0 != 0 || s1 != 1 {
		t.Fatalf("expected 0,1 got %d,%d", s0, s1)
	}
}

func TestBackwardManager_AddConn_And_GetDataChan(t *testing.T) {
	bm := newBackwardManager()
	bm.NewBackward("uuid1", "3000", "4000")
	seq := bm.GetNewSeq("uuid1", "4000")

	ok := bm.AddConn("uuid1", "4000", seq)
	if !ok {
		t.Fatal("AddConn should succeed")
	}

	ch, found := bm.GetDataChan("uuid1", "4000", seq)
	if !found || ch == nil {
		t.Fatal("GetDataChan should succeed")
	}

	ok = bm.AddConn("uuid1", "4000", 999)
	if ok {
		t.Fatal("AddConn should fail for unregistered seq")
	}

	_, found = bm.GetDataChan("uuid1", "4000", 999)
	if found {
		t.Fatal("GetDataChan should fail for unregistered seq")
	}
}

func TestBackwardManager_GetDataChanBySeq(t *testing.T) {
	bm := newBackwardManager()
	bm.NewBackward("uuid1", "3000", "4000")
	seq := bm.GetNewSeq("uuid1", "4000")
	bm.AddConn("uuid1", "4000", seq)

	ch, ok := bm.GetDataChanBySeq(seq)
	if !ok || ch == nil {
		t.Fatal("GetDataChanBySeq should succeed")
	}

	_, ok = bm.GetDataChanBySeq(999)
	if ok {
		t.Fatal("GetDataChanBySeq should fail for unknown seq")
	}
}

func TestBackwardManager_CheckBackward(t *testing.T) {
	bm := newBackwardManager()
	bm.NewBackward("uuid1", "3000", "4000")
	seq := bm.GetNewSeq("uuid1", "4000")
	bm.AddConn("uuid1", "4000", seq)

	if !bm.CheckBackward("uuid1", "4000", seq) {
		t.Fatal("CheckBackward should return true")
	}
	if bm.CheckBackward("uuid1", "4000", 999) {
		t.Fatal("CheckBackward should return false for unregistered seq")
	}
}

func TestBackwardManager_CloseTCP(t *testing.T) {
	bm := newBackwardManager()
	bm.NewBackward("uuid1", "3000", "4000")
	seq := bm.GetNewSeq("uuid1", "4000")
	bm.AddConn("uuid1", "4000", seq)

	bm.CloseTCP(seq)

	_, ok := bm.GetDataChanBySeq(seq)
	if ok {
		t.Fatal("data chan should be gone after CloseTCP")
	}

	// unknown seq should not panic
	bm.CloseTCP(999)
}

func TestBackwardManager_GetBackwardInfo(t *testing.T) {
	bm := newBackwardManager()
	bm.NewBackward("uuid1", "3000", "4000")

	infos, ok := bm.GetBackwardInfo("uuid1")
	if !ok || len(infos) != 1 {
		t.Fatal("GetBackwardInfo should return 1 entry")
	}
	if infos[0].LPort != "3000" || infos[0].RPort != "4000" {
		t.Fatalf("unexpected info: %+v", infos[0])
	}

	_, ok = bm.GetBackwardInfo("nonexistent")
	if ok {
		t.Fatal("GetBackwardInfo should fail for unknown uuid")
	}
}

func TestBackwardManager_GetStopRPort(t *testing.T) {
	bm := newBackwardManager()
	bm.NewBackward("uuid1", "3000", "4000")

	infos, _ := bm.GetBackwardInfo("uuid1")
	rPort := bm.GetStopRPort(infos[0].Seq)
	if rPort != "4000" {
		t.Fatalf("expected 4000, got %s", rPort)
	}
}

func TestBackwardManager_CloseSingle(t *testing.T) {
	bm := newBackwardManager()
	bm.NewBackward("uuid1", "3000", "4000")
	seq := bm.GetNewSeq("uuid1", "4000")
	bm.AddConn("uuid1", "4000", seq)

	bm.CloseSingle("uuid1", "4000")

	_, ok := bm.GetBackwardInfo("uuid1")
	if ok {
		t.Fatal("backward should be removed after CloseSingle")
	}
}

func TestBackwardManager_CloseSingleAll(t *testing.T) {
	bm := newBackwardManager()
	bm.NewBackward("uuid1", "3000", "4000")
	bm.NewBackward("uuid1", "3001", "4001")
	bm.GetNewSeq("uuid1", "4000")
	bm.GetNewSeq("uuid1", "4001")

	bm.CloseSingleAll("uuid1")

	_, ok := bm.GetBackwardInfo("uuid1")
	if ok {
		t.Fatal("all backwards should be removed")
	}
}

func TestBackwardManager_ForceShutdown(t *testing.T) {
	bm := newBackwardManager()
	bm.NewBackward("uuid1", "3000", "4000")
	seq := bm.GetNewSeq("uuid1", "4000")
	bm.AddConn("uuid1", "4000", seq)

	bm.ForceShutdown("uuid1")

	_, ok := bm.GetBackwardInfo("uuid1")
	if ok {
		t.Fatal("backward should be removed after ForceShutdown")
	}

	// missing uuid should not panic
	bm.ForceShutdown("nonexistent")
}

func TestBackwardManager_Concurrent(t *testing.T) {
	bm := newBackwardManager()
	bm.NewBackward("uuid1", "3000", "4000")

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = bm.GetNewSeq("uuid1", "4000")
		}()
	}
	wg.Wait()
}

// ===========================================================================
// FileManager tests
// ===========================================================================

func TestFileManager_NewTransfer(t *testing.T) {
	fm := newFileManager()
	f1 := fm.NewTransfer()
	f2 := fm.NewTransfer()

	if f1.TransferID == f2.TransferID {
		t.Fatal("transfer IDs should be unique")
	}
	if f1.TransferID != 1 || f2.TransferID != 2 {
		t.Fatalf("expected IDs 1,2 got %d,%d", f1.TransferID, f2.TransferID)
	}
}

func TestFileManager_GetTransfer(t *testing.T) {
	fm := newFileManager()
	f := fm.NewTransfer()

	got, ok := fm.GetTransfer(f.TransferID)
	if !ok || got != f {
		t.Fatal("GetTransfer should return the stored file")
	}

	_, ok = fm.GetTransfer(999)
	if ok {
		t.Fatal("GetTransfer should fail for unknown ID")
	}
}

func TestFileManager_StoreTransfer(t *testing.T) {
	fm := newFileManager()
	f := fm.NewTransfer()

	f2 := fm.NewTransfer()
	fm.StoreTransfer(f.TransferID, f2)

	got, _ := fm.GetTransfer(f.TransferID)
	if got != f2 {
		t.Fatal("StoreTransfer should overwrite")
	}
}

func TestFileManager_RemoveTransfer(t *testing.T) {
	fm := newFileManager()
	f := fm.NewTransfer()
	fm.RemoveTransfer(f.TransferID)

	_, ok := fm.GetTransfer(f.TransferID)
	if ok {
		t.Fatal("transfer should be removed")
	}
}

func TestFileManager_Concurrent(t *testing.T) {
	fm := newFileManager()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			f := fm.NewTransfer()
			fm.GetTransfer(f.TransferID)
		}()
	}
	wg.Wait()
}

// ===========================================================================
// ConsoleManager tests
// ===========================================================================

func TestConsoleManager(t *testing.T) {
	cm := newConsoleManager()
	if cm.OK == nil || cm.Exit == nil {
		t.Fatal("channels should be initialized")
	}
}

// ===========================================================================
// Channel-only managers (ssh, sshTunnel, shell, info, listen, connect,
// children, transport) — verify initialization
// ===========================================================================

func TestChannelManagers(t *testing.T) {
	if newSSHManager().SSHMessChan == nil {
		t.Error("SSHManager.SSHMessChan nil")
	}
	if newSSHTunnelManager().SSHTunnelMessChan == nil {
		t.Error("SSHTunnelManager.SSHTunnelMessChan nil")
	}
	if newShellManager().ShellMessChan == nil {
		t.Error("ShellManager.ShellMessChan nil")
	}
	if newInfoManager().InfoMessChan == nil {
		t.Error("InfoManager.InfoMessChan nil")
	}
	lm := newListenManager()
	if lm.ListenMessChan == nil || lm.ListenReady == nil {
		t.Error("ListenManager channels nil")
	}
	cm := newConnectManager()
	if cm.ConnectMessChan == nil || cm.ConnectReady == nil {
		t.Error("ConnectManager channels nil")
	}
	if newchildrenManager().ChildrenMessChan == nil {
		t.Error("childrenManager.ChildrenMessChan nil")
	}
	if newTransportManager().TransportMessChan == nil {
		t.Error("TransportManager.TransportMessChan nil")
	}
}
