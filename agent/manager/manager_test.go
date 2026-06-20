package manager

import (
	"net"
	"sync"
	"testing"
)

// ---------- Manager ----------

func TestNewManager(t *testing.T) {
	mgr := NewManager()
	if mgr == nil {
		t.Fatal("NewManager returned nil")
	}
	if mgr.ChildrenManager == nil {
		t.Fatal("ChildrenManager is nil")
	}
	if mgr.FileManager == nil {
		t.Fatal("FileManager is nil")
	}
	if mgr.SocksManager == nil {
		t.Fatal("SocksManager is nil")
	}
	if mgr.ForwardManager == nil {
		t.Fatal("ForwardManager is nil")
	}
	if mgr.BackwardManager == nil {
		t.Fatal("BackwardManager is nil")
	}
	if mgr.SSHManager == nil {
		t.Fatal("SSHManager is nil")
	}
	if mgr.SSHTunnelManager == nil {
		t.Fatal("SSHTunnelManager is nil")
	}
	if mgr.ShellManager == nil {
		t.Fatal("ShellManager is nil")
	}
	if mgr.ListenManager == nil {
		t.Fatal("ListenManager is nil")
	}
	if mgr.ConnectManager == nil {
		t.Fatal("ConnectManager is nil")
	}
	if mgr.OfflineManager == nil {
		t.Fatal("OfflineManager is nil")
	}
}

func TestManagerRun(t *testing.T) {
	mgr := NewManager()
	// Run is currently a no-op; ensure it does not panic.
	mgr.Run()
}

// ---------- SocksManager ----------

func TestSocksManager_CheckTCP_Empty(t *testing.T) {
	m := newSocksManager()
	if m.CheckTCP(1) {
		t.Fatal("expected CheckTCP to return false on empty map")
	}
}

func TestSocksManager_GetTCPDataChan_CreatesEntry(t *testing.T) {
	m := newSocksManager()
	ch, exists := m.GetTCPDataChan(42)
	if exists {
		t.Fatal("expected exists=false for first call")
	}
	if ch == nil {
		t.Fatal("expected non-nil channel")
	}
	if !m.CheckTCP(42) {
		t.Fatal("expected CheckTCP=true after GetTCPDataChan")
	}
}

func TestSocksManager_GetTCPDataChan_ReturnsExisting(t *testing.T) {
	m := newSocksManager()
	ch1, _ := m.GetTCPDataChan(1)
	ch2, exists := m.GetTCPDataChan(1)
	if !exists {
		t.Fatal("expected exists=true on second call")
	}
	// Channels should be the same instance.
	ch1 <- []byte("test")
	data := <-ch2
	if string(data) != "test" {
		t.Fatal("channels are not the same instance")
	}
}

func TestSocksManager_CheckUDP(t *testing.T) {
	m := newSocksManager()
	// CheckUDP on nonexistent seq returns false.
	if m.CheckUDP(1) {
		t.Fatal("expected false for nonexistent seq")
	}
	// Create a TCP entry first, then CheckUDP should return true and set up UDP.
	m.GetTCPDataChan(1)
	if !m.CheckUDP(1) {
		t.Fatal("expected CheckUDP=true after TCP entry exists")
	}
}

func TestSocksManager_GetUDPChans(t *testing.T) {
	m := newSocksManager()
	// Not set up yet.
	_, _, ok := m.GetUDPChans(1)
	if ok {
		t.Fatal("expected ok=false for nonexistent entry")
	}
	// Create TCP + UDP.
	m.GetTCPDataChan(1)
	m.CheckUDP(1)

	dataChan, readyChan, ok := m.GetUDPChans(1)
	if !ok {
		t.Fatal("expected ok=true after CheckUDP")
	}
	if dataChan == nil || readyChan == nil {
		t.Fatal("expected non-nil UDP channels")
	}
}

func TestSocksManager_UpdateAndGetUDPHeader(t *testing.T) {
	m := newSocksManager()
	m.GetTCPDataChan(1)
	m.CheckUDP(1)

	m.UpdateUDPHeader(1, "10.0.0.1:5000", []byte{0x01, 0x02})

	header, ok := m.GetUDPHeader(1, "10.0.0.1:5000")
	if !ok {
		t.Fatal("expected header to be found")
	}
	if len(header) != 2 || header[0] != 0x01 || header[1] != 0x02 {
		t.Fatalf("unexpected header: %v", header)
	}

	// Missing addr
	_, ok = m.GetUDPHeader(1, "1.2.3.4:999")
	if ok {
		t.Fatal("expected ok=false for unknown addr")
	}

	// Missing seq
	_, ok = m.GetUDPHeader(999, "10.0.0.1:5000")
	if ok {
		t.Fatal("expected ok=false for unknown seq")
	}
}

func TestSocksManager_CloseTCP(t *testing.T) {
	m := newSocksManager()
	m.GetTCPDataChan(1)
	m.CloseTCP(1)
	if m.CheckTCP(1) {
		t.Fatal("expected seq removed after CloseTCP")
	}
}

func TestSocksManager_CloseTCP_WithUDP(t *testing.T) {
	m := newSocksManager()
	m.GetTCPDataChan(1)
	m.CheckUDP(1)
	// Should close both TCP and UDP channels without panic.
	m.CloseTCP(1)
	if m.CheckTCP(1) {
		t.Fatal("expected seq removed after CloseTCP")
	}
}

func TestSocksManager_CloseTCP_Nonexistent(t *testing.T) {
	m := newSocksManager()
	// Should not panic.
	m.CloseTCP(999)
}

func TestSocksManager_CheckSocksReady(t *testing.T) {
	m := newSocksManager()
	if !m.CheckSocksReady() {
		t.Fatal("expected ready when map is empty")
	}
	m.GetTCPDataChan(1)
	if m.CheckSocksReady() {
		t.Fatal("expected not ready when map has entries")
	}
}

func TestSocksManager_ForceShutdown(t *testing.T) {
	m := newSocksManager()
	m.GetTCPDataChan(1)
	m.GetTCPDataChan(2)
	m.CheckUDP(1) // seq 1 also has UDP
	m.ForceShutdown()
	if !m.CheckSocksReady() {
		t.Fatal("expected map empty after ForceShutdown")
	}
}

// ---------- ForwardManager ----------

func TestForwardManager_NewForward(t *testing.T) {
	m := newForwardManager()
	m.NewForward(1)
	if !m.CheckForward(1) {
		t.Fatal("expected CheckForward=true after NewForward")
	}
}

func TestForwardManager_GetDataChan(t *testing.T) {
	m := newForwardManager()
	_, ok := m.GetDataChan(1)
	if ok {
		t.Fatal("expected ok=false before NewForward")
	}
	m.NewForward(1)
	ch, ok := m.GetDataChan(1)
	if !ok {
		t.Fatal("expected ok=true after NewForward")
	}
	if ch == nil {
		t.Fatal("expected non-nil data channel")
	}
}

func TestForwardManager_CloseTCP(t *testing.T) {
	m := newForwardManager()
	m.NewForward(1)
	m.CloseTCP(1)
	if m.CheckForward(1) {
		t.Fatal("expected forward removed after CloseTCP")
	}
}

func TestForwardManager_CloseTCP_Nonexistent(t *testing.T) {
	m := newForwardManager()
	// Should not panic.
	m.CloseTCP(999)
}

func TestForwardManager_ForceShutdown(t *testing.T) {
	m := newForwardManager()
	m.NewForward(1)
	m.NewForward(2)
	m.NewForward(3)
	m.ForceShutdown()
	if m.CheckForward(1) || m.CheckForward(2) || m.CheckForward(3) {
		t.Fatal("expected all forwards removed after ForceShutdown")
	}
}

// ---------- BackwardManager ----------

func TestBackwardManager_NewBackward(t *testing.T) {
	m := newBackwardManager()
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	defer ln.Close()

	m.NewBackward("9999", ln)

	seqChan, ok := m.GetSeqChan("9999")
	if !ok {
		t.Fatal("expected GetSeqChan ok=true after NewBackward")
	}
	if seqChan == nil {
		t.Fatal("expected non-nil seqChan")
	}
}

func TestBackwardManager_AddConn(t *testing.T) {
	m := newBackwardManager()
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	defer ln.Close()

	m.NewBackward("9999", ln)

	ok := m.AddConn("9999", 1)
	if !ok {
		t.Fatal("expected AddConn to succeed")
	}

	// AddConn to nonexistent port fails.
	ok = m.AddConn("1111", 2)
	if ok {
		t.Fatal("expected AddConn to fail for nonexistent port")
	}
}

func TestBackwardManager_GetDataChan(t *testing.T) {
	m := newBackwardManager()
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	defer ln.Close()

	m.NewBackward("9999", ln)
	m.AddConn("9999", 1)

	ch, ok := m.GetDataChan("9999", 1)
	if !ok || ch == nil {
		t.Fatal("expected to get data channel")
	}

	// Nonexistent port.
	_, ok = m.GetDataChan("1111", 1)
	if ok {
		t.Fatal("expected ok=false for nonexistent port")
	}

	// Nonexistent seq.
	_, ok = m.GetDataChan("9999", 999)
	if ok {
		t.Fatal("expected ok=false for nonexistent seq")
	}
}

func TestBackwardManager_GetDataChanBySeq(t *testing.T) {
	m := newBackwardManager()
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	defer ln.Close()

	m.NewBackward("9999", ln)
	m.AddConn("9999", 42)

	ch, ok := m.GetDataChanBySeq(42)
	if !ok || ch == nil {
		t.Fatal("expected to get data channel by seq")
	}

	_, ok = m.GetDataChanBySeq(999)
	if ok {
		t.Fatal("expected ok=false for unknown seq")
	}
}

func TestBackwardManager_CloseTCP(t *testing.T) {
	m := newBackwardManager()
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	defer ln.Close()

	m.NewBackward("9999", ln)
	m.AddConn("9999", 1)

	m.CloseTCP(1)

	_, ok := m.GetDataChan("9999", 1)
	if ok {
		t.Fatal("expected data chan removed after CloseTCP")
	}

	// Closing nonexistent seq should not panic.
	m.CloseTCP(999)
}

func TestBackwardManager_CloseSingle(t *testing.T) {
	m := newBackwardManager()
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	// ln will be closed by CloseSingle

	m.NewBackward("9999", ln)
	m.AddConn("9999", 1)

	m.CloseSingle("9999")

	_, ok := m.GetSeqChan("9999")
	if ok {
		t.Fatal("expected backward removed after CloseSingle")
	}
}

func TestBackwardManager_ForceShutdown(t *testing.T) {
	m := newBackwardManager()
	ln1, _ := net.Listen("tcp", "127.0.0.1:0")
	ln2, _ := net.Listen("tcp", "127.0.0.1:0")

	m.NewBackward("9999", ln1)
	m.NewBackward("8888", ln2)
	m.AddConn("9999", 1)
	m.AddConn("8888", 2)

	m.ForceShutdown()

	_, ok := m.GetSeqChan("9999")
	if ok {
		t.Fatal("expected all backwards removed after ForceShutdown")
	}
	_, ok = m.GetSeqChan("8888")
	if ok {
		t.Fatal("expected all backwards removed after ForceShutdown")
	}
}

// ---------- ChildrenManager ----------

func TestChildrenManager_NewChild(t *testing.T) {
	m := newChildrenManager()
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	m.NewChild("uuid-1", c1, nil)

	conn, ok := m.GetConn("uuid-1")
	if !ok {
		t.Fatal("expected to find child")
	}
	if conn != c1 {
		t.Fatal("returned conn does not match")
	}
}

func TestChildrenManager_GetConn_Nonexistent(t *testing.T) {
	m := newChildrenManager()
	_, ok := m.GetConn("nonexistent")
	if ok {
		t.Fatal("expected ok=false for nonexistent child")
	}
}

func TestChildrenManager_DelChild(t *testing.T) {
	m := newChildrenManager()
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	m.NewChild("uuid-1", c1, nil)
	m.DelChild("uuid-1")

	_, ok := m.GetConn("uuid-1")
	if ok {
		t.Fatal("expected child removed after DelChild")
	}
}

func TestChildrenManager_DelChild_Nonexistent(t *testing.T) {
	m := newChildrenManager()
	// Should not panic.
	m.DelChild("nonexistent")
}

func TestChildrenManager_GetAllChildren(t *testing.T) {
	m := newChildrenManager()
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()
	c3, c4 := net.Pipe()
	defer c3.Close()
	defer c4.Close()

	m.NewChild("uuid-1", c1, nil)
	m.NewChild("uuid-2", c3, nil)

	all := m.GetAllChildren()
	if len(all) != 2 {
		t.Fatalf("expected 2 children, got %d", len(all))
	}

	found := make(map[string]bool)
	for _, uuid := range all {
		found[uuid] = true
	}
	if !found["uuid-1"] || !found["uuid-2"] {
		t.Fatal("expected both uuid-1 and uuid-2 in results")
	}
}

func TestChildrenManager_GetAllChildren_Empty(t *testing.T) {
	m := newChildrenManager()
	all := m.GetAllChildren()
	if len(all) != 0 {
		t.Fatalf("expected 0 children, got %d", len(all))
	}
}

func TestChildrenManager_OverwriteChild(t *testing.T) {
	m := newChildrenManager()
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()
	c3, c4 := net.Pipe()
	defer c3.Close()
	defer c4.Close()

	m.NewChild("uuid-1", c1, nil)
	m.NewChild("uuid-1", c3, nil) // overwrite

	conn, ok := m.GetConn("uuid-1")
	if !ok {
		t.Fatal("expected to find child after overwrite")
	}
	if conn != c3 {
		t.Fatal("expected overwritten conn")
	}
}

// ---------- Concurrency ----------

func TestSocksManager_Concurrent(t *testing.T) {
	m := newSocksManager()
	var wg sync.WaitGroup

	for i := uint64(0); i < 50; i++ {
		wg.Add(1)
		go func(seq uint64) {
			defer wg.Done()
			m.GetTCPDataChan(seq)
			m.CheckTCP(seq)
		}(i)
	}
	wg.Wait()

	for i := uint64(0); i < 50; i++ {
		if !m.CheckTCP(i) {
			t.Fatalf("expected seq %d to exist", i)
		}
	}
}

func TestForwardManager_Concurrent(t *testing.T) {
	m := newForwardManager()
	var wg sync.WaitGroup

	for i := uint64(0); i < 50; i++ {
		wg.Add(1)
		go func(seq uint64) {
			defer wg.Done()
			m.NewForward(seq)
			m.GetDataChan(seq)
			m.CheckForward(seq)
		}(i)
	}
	wg.Wait()

	for i := uint64(0); i < 50; i++ {
		if !m.CheckForward(i) {
			t.Fatalf("expected forward %d to exist", i)
		}
	}
}

func TestChildrenManager_Concurrent(t *testing.T) {
	m := newChildrenManager()
	var wg sync.WaitGroup

	conns := make([]net.Conn, 50)
	peers := make([]net.Conn, 50)
	for i := 0; i < 50; i++ {
		c1, c2 := net.Pipe()
		conns[i] = c1
		peers[i] = c2
	}
	defer func() {
		for i := 0; i < 50; i++ {
			conns[i].Close()
			peers[i].Close()
		}
	}()

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			uuid := string(rune('A'+idx)) + "-uuid"
			m.NewChild(uuid, conns[idx], nil)
			m.GetConn(uuid)
		}(i)
	}
	wg.Wait()
}

func TestBackwardManager_Concurrent(t *testing.T) {
	m := newBackwardManager()
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	defer ln.Close()

	m.NewBackward("7777", ln)

	var wg sync.WaitGroup
	for i := uint64(1); i <= 50; i++ {
		wg.Add(1)
		go func(seq uint64) {
			defer wg.Done()
			m.AddConn("7777", seq)
			m.GetDataChan("7777", seq)
			m.GetDataChanBySeq(seq)
		}(i)
	}
	wg.Wait()
}

// ---------- Channel message channels ----------

func TestManagerChannels_NotNil(t *testing.T) {
	mgr := NewManager()
	if mgr.SocksManager.SocksMessChan == nil {
		t.Fatal("SocksMessChan is nil")
	}
	if mgr.ForwardManager.ForwardMessChan == nil {
		t.Fatal("ForwardMessChan is nil")
	}
	if mgr.BackwardManager.BackwardMessChan == nil {
		t.Fatal("BackwardMessChan is nil")
	}
	if mgr.BackwardManager.SeqReady == nil {
		t.Fatal("SeqReady is nil")
	}
	if mgr.ChildrenManager.ChildComeChan == nil {
		t.Fatal("ChildComeChan is nil")
	}
	if mgr.SSHManager.SSHMessChan == nil {
		t.Fatal("SSHMessChan is nil")
	}
	if mgr.SSHTunnelManager.SSHTunnelMessChan == nil {
		t.Fatal("SSHTunnelMessChan is nil")
	}
	if mgr.ShellManager.ShellMessChan == nil {
		t.Fatal("ShellMessChan is nil")
	}
	if mgr.ListenManager.ListenMessChan == nil {
		t.Fatal("ListenMessChan is nil")
	}
	if mgr.ListenManager.ChildUUIDChan == nil {
		t.Fatal("ChildUUIDChan is nil")
	}
	if mgr.ConnectManager.ConnectMessChan == nil {
		t.Fatal("ConnectMessChan is nil")
	}
	if mgr.OfflineManager.OfflineMessChan == nil {
		t.Fatal("OfflineMessChan is nil")
	}
	if mgr.FileManager.FileMessChan == nil {
		t.Fatal("FileMessChan is nil")
	}
}
