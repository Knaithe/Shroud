package cli

import (
	"sort"
	"testing"

	"Shroud/protocol"
)

// ===========================================================================
// Helper (trie) tests
// ===========================================================================

func TestNewHelper(t *testing.T) {
	h := NewHelper()
	if h == nil {
		t.Fatal("NewHelper returned nil")
	}
	if h.adminTree == nil || h.adminTree.root == nil {
		t.Error("adminTree not initialized")
	}
	if h.nodeTree == nil || h.nodeTree.root == nil {
		t.Error("nodeTree not initialized")
	}
	if h.TaskChan == nil {
		t.Error("TaskChan is nil")
	}
	if h.ResultChan == nil {
		t.Error("ResultChan is nil")
	}
	if h.min != 0 {
		t.Errorf("expected min=0, got %d", h.min)
	}
	if h.max != 12 {
		t.Errorf("expected max=12, got %d", h.max)
	}
}

func TestHelper_AdminCommands(t *testing.T) {
	expected := []string{"use", "detail", "topo", "resettoken", "help", "exit"}
	h := NewHelper()
	if len(h.adminList) != len(expected) {
		t.Fatalf("expected %d admin commands, got %d", len(expected), len(h.adminList))
	}
	for i, cmd := range expected {
		if h.adminList[i] != cmd {
			t.Errorf("admin command %d: expected '%s', got '%s'", i, cmd, h.adminList[i])
		}
	}
}

func TestHelper_NodeCommands(t *testing.T) {
	h := NewHelper()
	if len(h.nodeList) == 0 {
		t.Fatal("nodeList should not be empty")
	}
	// Verify a few key commands exist.
	nodeSet := make(map[string]bool)
	for _, cmd := range h.nodeList {
		nodeSet[cmd] = true
	}
	for _, cmd := range []string{"ssh", "shell", "socks", "connect", "upload", "download", "exit"} {
		if !nodeSet[cmd] {
			t.Errorf("expected node command '%s' not found", cmd)
		}
	}
}

// helperWithTries creates a Helper and populates both tries (simulating Run
// without blocking on the channel loop).
func helperWithTries() *Helper {
	h := NewHelper()
	h.insertAdmin()
	h.insertNode()
	return h
}

func TestHelper_SearchAdminExactMatch(t *testing.T) {
	h := helperWithTries()

	task := &HelperTask{IsNodeMode: false, Uncomplete: "help"}
	result := h.search(task)
	if len(result) != 1 {
		t.Fatalf("expected 1 result for 'help', got %d: %v", len(result), result)
	}
	if result[0] != "help" {
		t.Errorf("expected 'help', got '%s'", result[0])
	}
}

func TestHelper_SearchAdminPrefix(t *testing.T) {
	h := helperWithTries()

	// "e" should match "exit"
	task := &HelperTask{IsNodeMode: false, Uncomplete: "e"}
	result := h.search(task)
	if len(result) != 1 {
		t.Fatalf("expected 1 result for prefix 'e', got %d: %v", len(result), result)
	}
	if result[0] != "exit" {
		t.Errorf("expected 'exit', got '%s'", result[0])
	}
}

func TestHelper_SearchAdminNoMatch(t *testing.T) {
	h := helperWithTries()

	task := &HelperTask{IsNodeMode: false, Uncomplete: "zzz"}
	result := h.search(task)
	if len(result) != 0 {
		t.Fatalf("expected 0 results for 'zzz', got %d: %v", len(result), result)
	}
}

func TestHelper_SearchAdminEmptyInput(t *testing.T) {
	h := helperWithTries()

	// Empty string has len 0 which equals min(0), so it enters the loop
	// but samePrefix stays empty, so no results.
	task := &HelperTask{IsNodeMode: false, Uncomplete: ""}
	result := h.search(task)
	if len(result) != 0 {
		t.Fatalf("expected 0 results for empty input, got %d", len(result))
	}
}

func TestHelper_SearchAdminTooLong(t *testing.T) {
	h := helperWithTries()

	// Input longer than max (12) should return empty.
	task := &HelperTask{IsNodeMode: false, Uncomplete: "abcdefghijklm"}
	result := h.search(task)
	if len(result) != 0 {
		t.Fatalf("expected 0 results for too-long input, got %d", len(result))
	}
}

func TestHelper_SearchNodePrefix(t *testing.T) {
	h := helperWithTries()

	// "sh" should match "shell", "shutdown", "shell" is one, and "shutdown" is another.
	task := &HelperTask{IsNodeMode: true, Uncomplete: "sh"}
	result := h.search(task)

	// Should include: shell, shutdown
	sort.Strings(result)
	found := make(map[string]bool)
	for _, r := range result {
		found[r] = true
	}
	if !found["shell"] {
		t.Error("expected 'shell' in results")
	}
	if !found["shutdown"] {
		t.Error("expected 'shutdown' in results")
	}
}

func TestHelper_SearchNodeSocksPrefix(t *testing.T) {
	h := helperWithTries()

	// "so" should match "socks"
	task := &HelperTask{IsNodeMode: true, Uncomplete: "so"}
	result := h.search(task)
	found := false
	for _, r := range result {
		if r == "socks" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'socks' in results, got: %v", result)
	}
}

func TestHelper_SearchNodeSSHPrefix(t *testing.T) {
	h := helperWithTries()

	// "ssh" should match "ssh" and "sshtunnel"
	task := &HelperTask{IsNodeMode: true, Uncomplete: "ssh"}
	result := h.search(task)
	sort.Strings(result)

	found := make(map[string]bool)
	for _, r := range result {
		found[r] = true
	}
	if !found["ssh"] {
		t.Error("expected 'ssh' in results")
	}
	if !found["sshtunnel"] {
		t.Error("expected 'sshtunnel' in results")
	}
}

func TestHelper_SearchResultsSorted(t *testing.T) {
	h := helperWithTries()

	task := &HelperTask{IsNodeMode: true, Uncomplete: "s"}
	result := h.search(task)

	if !sort.StringsAreSorted(result) {
		t.Errorf("results should be sorted, got: %v", result)
	}
}

func TestHelper_RunChannel(t *testing.T) {
	h := NewHelper()
	go h.Run()

	// Send a task and read result.
	h.TaskChan <- &HelperTask{IsNodeMode: false, Uncomplete: "to"}
	result := <-h.ResultChan

	if len(result) != 1 || result[0] != "topo" {
		t.Errorf("expected ['topo'], got %v", result)
	}
}

// ===========================================================================
// NewConsole tests
// ===========================================================================

func TestNewConsole(t *testing.T) {
	c := NewConsole()
	if c == nil {
		t.Fatal("NewConsole returned nil")
	}
	if c.status != "(admin) >> " {
		t.Errorf("expected status '(admin) >> ', got '%s'", c.status)
	}
	if c.ready == nil {
		t.Error("ready channel is nil")
	}
	if c.getCommand == nil {
		t.Error("getCommand channel is nil")
	}
	if c.shellMode {
		t.Error("shellMode should be false initially")
	}
	if c.sshMode {
		t.Error("sshMode should be false initially")
	}
	if c.nodeMode {
		t.Error("nodeMode should be false initially")
	}
}

// ===========================================================================
// NewHistory tests
// ===========================================================================

func TestNewHistory(t *testing.T) {
	h := NewHistory()
	if h == nil {
		t.Fatal("NewHistory returned nil")
	}
	if h.normal == nil || h.normal.storeList == nil {
		t.Error("normal history list not initialized")
	}
	if h.shell == nil || h.shell.storeList == nil {
		t.Error("shell history list not initialized")
	}
	if h.ssh == nil || h.ssh.storeList == nil {
		t.Error("ssh history list not initialized")
	}
	if h.normal.capacity != 100 {
		t.Errorf("expected normal capacity 100, got %d", h.normal.capacity)
	}
	if h.TaskChan == nil {
		t.Error("TaskChan is nil")
	}
	if h.ResultChan == nil {
		t.Error("ResultChan is nil")
	}
}

func TestHistory_RecordAndSearch(t *testing.T) {
	h := NewHistory()
	go h.Run()

	// Record a command.
	h.TaskChan <- &HistoryTask{
		Mode:    RECORD,
		Type:    NORMAL,
		Command: "first command",
	}

	// Record a second command.
	h.TaskChan <- &HistoryTask{
		Mode:    RECORD,
		Type:    NORMAL,
		Command: "second command",
	}

	// Search BEGIN (should return most recent = "second command").
	h.TaskChan <- &HistoryTask{
		Mode:  SEARCH,
		Type:  NORMAL,
		Order: BEGIN,
	}
	result := <-h.ResultChan
	if result != "second command" {
		t.Errorf("expected 'second command', got '%s'", result)
	}

	// Search NEXT (should return "first command").
	h.TaskChan <- &HistoryTask{
		Mode:  SEARCH,
		Type:  NORMAL,
		Order: NEXT,
	}
	result = <-h.ResultChan
	if result != "first command" {
		t.Errorf("expected 'first command', got '%s'", result)
	}

	// Search PREV (should go back to "second command").
	h.TaskChan <- &HistoryTask{
		Mode:  SEARCH,
		Type:  NORMAL,
		Order: PREV,
	}
	result = <-h.ResultChan
	if result != "second command" {
		t.Errorf("expected 'second command', got '%s'", result)
	}
}

func TestHistory_EmptySearch(t *testing.T) {
	h := NewHistory()
	go h.Run()

	// Search on empty history should return empty string.
	h.TaskChan <- &HistoryTask{
		Mode:  SEARCH,
		Type:  NORMAL,
		Order: BEGIN,
	}
	result := <-h.ResultChan
	if result != "" {
		t.Errorf("expected empty string for empty history, got '%s'", result)
	}
}

func TestHistory_ShellType(t *testing.T) {
	h := NewHistory()
	go h.Run()

	h.TaskChan <- &HistoryTask{
		Mode:    RECORD,
		Type:    SHELL,
		Command: "ls -la",
	}

	h.TaskChan <- &HistoryTask{
		Mode:  SEARCH,
		Type:  SHELL,
		Order: BEGIN,
	}
	result := <-h.ResultChan
	if result != "ls -la" {
		t.Errorf("expected 'ls -la', got '%s'", result)
	}
}

func TestHistory_SSHType(t *testing.T) {
	h := NewHistory()
	go h.Run()

	h.TaskChan <- &HistoryTask{
		Mode:    RECORD,
		Type:    SSH,
		Command: "whoami",
	}

	h.TaskChan <- &HistoryTask{
		Mode:  SEARCH,
		Type:  SSH,
		Order: BEGIN,
	}
	result := <-h.ResultChan
	if result != "whoami" {
		t.Errorf("expected 'whoami', got '%s'", result)
	}
}

// ===========================================================================
// Constants / Banner tests
// ===========================================================================

func TestConstants(t *testing.T) {
	if MAIN != 0 {
		t.Errorf("MAIN expected 0, got %d", MAIN)
	}
	if NODE != 1 {
		t.Errorf("NODE expected 1, got %d", NODE)
	}
}

func TestKeyConstants(t *testing.T) {
	if KeyNone != 0 {
		t.Errorf("KeyNone expected 0, got %d", KeyNone)
	}
	if KeyBackspace != 1 {
		t.Errorf("KeyBackspace expected 1, got %d", KeyBackspace)
	}
	if KeyEnter != 2 {
		t.Errorf("KeyEnter expected 2, got %d", KeyEnter)
	}
}

func TestShroudVersion(t *testing.T) {
	if protocol.SHROUD_VERSION == "" {
		t.Error("SHROUD_VERSION should not be empty")
	}
}

func TestBanner_NoPanic(t *testing.T) {
	// Calling Banner should not panic.
	Banner()
}

func TestShowMainHelp_NoPanic(t *testing.T) {
	ShowMainHelp()
}

func TestShowNodeHelp_NoPanic(t *testing.T) {
	ShowNodeHelp()
}

func TestHistoryConstants(t *testing.T) {
	// All constants are in a single iota block in history.go:
	// RECORD=0, SEARCH=1, NORMAL=2, SHELL=3, SSH=4, BEGIN=5, PREV=6, NEXT=7
	if RECORD != 0 {
		t.Errorf("RECORD expected 0, got %d", RECORD)
	}
	if SEARCH != 1 {
		t.Errorf("SEARCH expected 1, got %d", SEARCH)
	}
	if NORMAL != 2 {
		t.Errorf("NORMAL expected 2, got %d", NORMAL)
	}
	if SHELL != 3 {
		t.Errorf("SHELL expected 3, got %d", SHELL)
	}
	if SSH != 4 {
		t.Errorf("SSH expected 4, got %d", SSH)
	}
	if BEGIN != 5 {
		t.Errorf("BEGIN expected 5, got %d", BEGIN)
	}
	if PREV != 6 {
		t.Errorf("PREV expected 6, got %d", PREV)
	}
	if NEXT != 7 {
		t.Errorf("NEXT expected 7, got %d", NEXT)
	}
}
