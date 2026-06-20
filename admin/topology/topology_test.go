package topology

import (
	"Shroud/admin/printer"
	"Shroud/protocol"
	"sync"
	"testing"
)

var initPrinterOnce sync.Once

// helper: create a Topology with Run() goroutine started
func setupTopology(t *testing.T) *Topology {
	t.Helper()
	initPrinterOnce.Do(printer.InitPrinter)
	topo := NewTopology()
	go topo.Run()
	return topo
}

func TestNewTopology(t *testing.T) {
	topo := NewTopology()
	if topo == nil {
		t.Fatal("NewTopology returned nil")
	}
	if topo.currentIDNum != 0 {
		t.Errorf("expected currentIDNum 0, got %d", topo.currentIDNum)
	}
	if topo.nodes == nil {
		t.Error("nodes map is nil")
	}
	if topo.route == nil {
		t.Error("route map is nil")
	}
	if topo.history == nil {
		t.Error("history map is nil")
	}
	if topo.TaskChan == nil {
		t.Error("TaskChan is nil")
	}
	if topo.ResultChan == nil {
		t.Error("ResultChan is nil")
	}
}

func TestNewNode(t *testing.T) {
	n := NewNode("testuuid01", "192.168.1.1")
	if n == nil {
		t.Fatal("NewNode returned nil")
	}
	if n.uuid != "testuuid01" {
		t.Errorf("expected uuid 'testuuid01', got '%s'", n.uuid)
	}
	if n.currentIP != "192.168.1.1" {
		t.Errorf("expected ip '192.168.1.1', got '%s'", n.currentIP)
	}
}

func TestAddNodeFirst(t *testing.T) {
	topo := setupTopology(t)

	n := NewNode("node000001", "10.0.0.1")
	task := &TopoTask{
		Mode:    ADDNODE,
		Target:  n,
		IsFirst: true,
	}
	topo.TaskChan <- task
	result := <-topo.ResultChan

	if result.IDNum != 0 {
		t.Errorf("expected IDNum 0 for first node, got %d", result.IDNum)
	}
}

func TestAddNodeChild(t *testing.T) {
	topo := setupTopology(t)

	// Add parent (first node)
	parent := NewNode("parentuuid0", "10.0.0.1")
	topo.TaskChan <- &TopoTask{
		Mode:    ADDNODE,
		Target:  parent,
		IsFirst: true,
	}
	<-topo.ResultChan

	// Add child node
	child := NewNode("childuuid00", "10.0.0.2")
	topo.TaskChan <- &TopoTask{
		Mode:       ADDNODE,
		Target:     child,
		ParentUUID: "parentuuid0",
		IsFirst:    false,
	}
	result := <-topo.ResultChan

	if result.IDNum != 1 {
		t.Errorf("expected IDNum 1 for second node, got %d", result.IDNum)
	}
}

func TestGetUUID(t *testing.T) {
	topo := setupTopology(t)

	n := NewNode("uuid012345", "10.0.0.1")
	topo.TaskChan <- &TopoTask{
		Mode:    ADDNODE,
		Target:  n,
		IsFirst: true,
	}
	<-topo.ResultChan

	topo.TaskChan <- &TopoTask{
		Mode:    GETUUID,
		UUIDNum: 0,
	}
	result := <-topo.ResultChan

	if result.UUID != "uuid012345" {
		t.Errorf("expected uuid 'uuid012345', got '%s'", result.UUID)
	}
}

func TestGetUUIDNum(t *testing.T) {
	topo := setupTopology(t)

	n := NewNode("uuid012345", "10.0.0.1")
	topo.TaskChan <- &TopoTask{
		Mode:    ADDNODE,
		Target:  n,
		IsFirst: true,
	}
	<-topo.ResultChan

	topo.TaskChan <- &TopoTask{
		Mode: GETUUIDNUM,
		UUID: "uuid012345",
	}
	result := <-topo.ResultChan

	if result.IDNum != 0 {
		t.Errorf("expected IDNum 0, got %d", result.IDNum)
	}
}

func TestGetUUIDNumNotFound(t *testing.T) {
	topo := setupTopology(t)

	n := NewNode("uuid012345", "10.0.0.1")
	topo.TaskChan <- &TopoTask{
		Mode:    ADDNODE,
		Target:  n,
		IsFirst: true,
	}
	<-topo.ResultChan

	topo.TaskChan <- &TopoTask{
		Mode: GETUUIDNUM,
		UUID: "nonexist00",
	}
	result := <-topo.ResultChan

	if result.IDNum != -1 {
		t.Errorf("expected IDNum -1 for missing uuid, got %d", result.IDNum)
	}
}

func TestCheckNodeExists(t *testing.T) {
	topo := setupTopology(t)

	n := NewNode("uuid012345", "10.0.0.1")
	topo.TaskChan <- &TopoTask{
		Mode:    ADDNODE,
		Target:  n,
		IsFirst: true,
	}
	<-topo.ResultChan

	topo.TaskChan <- &TopoTask{
		Mode:    CHECKNODE,
		UUIDNum: 0,
	}
	result := <-topo.ResultChan

	if !result.IsExist {
		t.Error("expected node 0 to exist")
	}
}

func TestCheckNodeNotExists(t *testing.T) {
	topo := setupTopology(t)

	topo.TaskChan <- &TopoTask{
		Mode:    CHECKNODE,
		UUIDNum: 99,
	}
	result := <-topo.ResultChan

	if result.IsExist {
		t.Error("expected node 99 to not exist")
	}
}

func TestCalculateRouteFirstNode(t *testing.T) {
	topo := setupTopology(t)

	n := NewNode("node000001", "10.0.0.1")
	n.parentUUID = protocol.ADMIN_UUID
	topo.TaskChan <- &TopoTask{
		Mode:    ADDNODE,
		Target:  n,
		IsFirst: true,
	}
	<-topo.ResultChan

	topo.TaskChan <- &TopoTask{Mode: CALCULATE}
	<-topo.ResultChan

	topo.TaskChan <- &TopoTask{
		Mode: GETROUTE,
		UUID: "node000001",
	}
	result := <-topo.ResultChan

	// First node's route is its own UUID (first-hop to reach it is itself)
	if result.Route != "node000001" {
		t.Errorf("expected route 'node000001' for first node, got '%s'", result.Route)
	}
}

func TestCalculateRouteChildNode(t *testing.T) {
	topo := setupTopology(t)

	// Add parent (first node, directly connected to admin)
	parent := NewNode("parentuuid0", "10.0.0.1")
	topo.TaskChan <- &TopoTask{
		Mode:    ADDNODE,
		Target:  parent,
		IsFirst: true,
	}
	<-topo.ResultChan

	// Add child node (connected through parent)
	child := NewNode("childuuid00", "10.0.0.2")
	topo.TaskChan <- &TopoTask{
		Mode:       ADDNODE,
		Target:     child,
		ParentUUID: "parentuuid0",
		IsFirst:    false,
	}
	<-topo.ResultChan

	// Calculate routes
	topo.TaskChan <- &TopoTask{Mode: CALCULATE}
	<-topo.ResultChan

	// Get child's route
	topo.TaskChan <- &TopoTask{
		Mode: GETROUTE,
		UUID: "childuuid00",
	}
	result := <-topo.ResultChan

	// Child's first-hop route should be the parent (direct child of admin)
	if result.Route != "parentuuid0" {
		t.Errorf("expected route 'parentuuid0', got '%s'", result.Route)
	}
}

func TestCalculateRouteGrandchild(t *testing.T) {
	topo := setupTopology(t)

	// Add parent (first node)
	parent := NewNode("parentuuid0", "10.0.0.1")
	topo.TaskChan <- &TopoTask{
		Mode:    ADDNODE,
		Target:  parent,
		IsFirst: true,
	}
	<-topo.ResultChan

	// Add child
	child := NewNode("childuuid00", "10.0.0.2")
	topo.TaskChan <- &TopoTask{
		Mode:       ADDNODE,
		Target:     child,
		ParentUUID: "parentuuid0",
		IsFirst:    false,
	}
	<-topo.ResultChan

	// Add grandchild
	grandchild := NewNode("grandchild0", "10.0.0.3")
	topo.TaskChan <- &TopoTask{
		Mode:       ADDNODE,
		Target:     grandchild,
		ParentUUID: "childuuid00",
		IsFirst:    false,
	}
	<-topo.ResultChan

	// Calculate routes
	topo.TaskChan <- &TopoTask{Mode: CALCULATE}
	<-topo.ResultChan

	// Get grandchild's route
	topo.TaskChan <- &TopoTask{
		Mode: GETROUTE,
		UUID: "grandchild0",
	}
	result := <-topo.ResultChan

	// Grandchild's first-hop route should be the same first-hop (parentuuid0)
	if result.Route != "parentuuid0" {
		t.Errorf("expected route 'parentuuid0', got '%s'", result.Route)
	}
}

func TestUpdateDetail(t *testing.T) {
	topo := setupTopology(t)

	n := NewNode("uuid012345", "10.0.0.1")
	topo.TaskChan <- &TopoTask{
		Mode:    ADDNODE,
		Target:  n,
		IsFirst: true,
	}
	<-topo.ResultChan

	topo.TaskChan <- &TopoTask{
		Mode:     UPDATEDETAIL,
		UUID:     "uuid012345",
		UserName: "testuser",
		HostName: "testhost",
		Memo:     "test memo",
	}
	// updateDetail does not send result, so verify directly
	// We need to verify via showDetail, but that prints to stdout.
	// Instead, verify the internal state directly.
	// Since updateDetail doesn't send to ResultChan, we synchronize
	// by sending another task that does respond.
	topo.TaskChan <- &TopoTask{
		Mode:    CHECKNODE,
		UUIDNum: 0,
	}
	<-topo.ResultChan

	// Access internal state (same package)
	if topo.nodes[0].currentUser != "testuser" {
		t.Errorf("expected user 'testuser', got '%s'", topo.nodes[0].currentUser)
	}
	if topo.nodes[0].currentHostname != "testhost" {
		t.Errorf("expected hostname 'testhost', got '%s'", topo.nodes[0].currentHostname)
	}
	if topo.nodes[0].memo != "test memo" {
		t.Errorf("expected memo 'test memo', got '%s'", topo.nodes[0].memo)
	}
}

func TestUpdateMemo(t *testing.T) {
	topo := setupTopology(t)

	n := NewNode("uuid012345", "10.0.0.1")
	topo.TaskChan <- &TopoTask{
		Mode:    ADDNODE,
		Target:  n,
		IsFirst: true,
	}
	<-topo.ResultChan

	topo.TaskChan <- &TopoTask{
		Mode: UPDATEMEMO,
		UUID: "uuid012345",
		Memo: "updated memo",
	}
	// Synchronize
	topo.TaskChan <- &TopoTask{
		Mode:    CHECKNODE,
		UUIDNum: 0,
	}
	<-topo.ResultChan

	if topo.nodes[0].memo != "updated memo" {
		t.Errorf("expected memo 'updated memo', got '%s'", topo.nodes[0].memo)
	}
}

func TestDelNode(t *testing.T) {
	topo := setupTopology(t)

	// Add parent
	parent := NewNode("parentuuid0", "10.0.0.1")
	topo.TaskChan <- &TopoTask{
		Mode:    ADDNODE,
		Target:  parent,
		IsFirst: true,
	}
	<-topo.ResultChan

	// Add child
	child := NewNode("childuuid00", "10.0.0.2")
	topo.TaskChan <- &TopoTask{
		Mode:       ADDNODE,
		Target:     child,
		ParentUUID: "parentuuid0",
		IsFirst:    false,
	}
	<-topo.ResultChan

	// Delete child
	topo.TaskChan <- &TopoTask{
		Mode: DELNODE,
		UUID: "childuuid00",
	}
	result := <-topo.ResultChan

	if len(result.AllNodes) != 1 {
		t.Errorf("expected 1 deleted node, got %d", len(result.AllNodes))
	}
	if result.AllNodes[0] != "childuuid00" {
		t.Errorf("expected deleted node 'childuuid00', got '%s'", result.AllNodes[0])
	}

	// Verify node no longer exists
	topo.TaskChan <- &TopoTask{
		Mode:    CHECKNODE,
		UUIDNum: 1,
	}
	checkResult := <-topo.ResultChan
	if checkResult.IsExist {
		t.Error("deleted node should not exist")
	}
}

func TestDelNodeCascade(t *testing.T) {
	topo := setupTopology(t)

	// Build chain: parent -> child -> grandchild
	parent := NewNode("parentuuid0", "10.0.0.1")
	topo.TaskChan <- &TopoTask{
		Mode:    ADDNODE,
		Target:  parent,
		IsFirst: true,
	}
	<-topo.ResultChan

	child := NewNode("childuuid00", "10.0.0.2")
	topo.TaskChan <- &TopoTask{
		Mode:       ADDNODE,
		Target:     child,
		ParentUUID: "parentuuid0",
		IsFirst:    false,
	}
	<-topo.ResultChan

	grandchild := NewNode("grandchild0", "10.0.0.3")
	topo.TaskChan <- &TopoTask{
		Mode:       ADDNODE,
		Target:     grandchild,
		ParentUUID: "childuuid00",
		IsFirst:    false,
	}
	<-topo.ResultChan

	// Delete child (should cascade to grandchild)
	topo.TaskChan <- &TopoTask{
		Mode: DELNODE,
		UUID: "childuuid00",
	}
	result := <-topo.ResultChan

	if len(result.AllNodes) != 2 {
		t.Errorf("expected 2 deleted nodes (child + grandchild), got %d", len(result.AllNodes))
	}
}

func TestReonlineNodeExisting(t *testing.T) {
	topo := setupTopology(t)

	// Add and then delete a node
	n := NewNode("uuid012345", "10.0.0.1")
	topo.TaskChan <- &TopoTask{
		Mode:    ADDNODE,
		Target:  n,
		IsFirst: true,
	}
	addResult := <-topo.ResultChan
	originalIDNum := addResult.IDNum

	// Add a second node so delNode can find parent's children
	n2 := NewNode("uuid067890", "10.0.0.2")
	topo.TaskChan <- &TopoTask{
		Mode:       ADDNODE,
		Target:     n2,
		ParentUUID: "uuid012345",
		IsFirst:    false,
	}
	<-topo.ResultChan

	// Delete the second node
	topo.TaskChan <- &TopoTask{
		Mode: DELNODE,
		UUID: "uuid067890",
	}
	<-topo.ResultChan

	// Re-online the deleted node (should reuse history IDNum)
	reNode := NewNode("uuid067890", "10.0.0.2")
	topo.TaskChan <- &TopoTask{
		Mode:       REONLINENODE,
		Target:     reNode,
		ParentUUID: "uuid012345",
		IsFirst:    false,
	}
	<-topo.ResultChan

	// Check it exists again
	topo.TaskChan <- &TopoTask{
		Mode:    CHECKNODE,
		UUIDNum: 1, // should be re-added at IDNum 1
	}
	checkResult := <-topo.ResultChan
	if !checkResult.IsExist {
		t.Error("re-onlined node should exist")
	}

	_ = originalIDNum
}

func TestReonlineNodeFirst(t *testing.T) {
	topo := setupTopology(t)

	// Re-online as first node
	n := NewNode("uuid012345", "10.0.0.1")
	topo.TaskChan <- &TopoTask{
		Mode:    REONLINENODE,
		Target:  n,
		IsFirst: true,
	}
	<-topo.ResultChan

	// Verify it got assigned ADMIN_UUID as parent
	topo.TaskChan <- &TopoTask{
		Mode:    CHECKNODE,
		UUIDNum: 0,
	}
	result := <-topo.ResultChan
	if !result.IsExist {
		t.Error("re-onlined first node should exist at IDNum 0")
	}
}

func TestShowDetailDoesNotPanic(t *testing.T) {
	topo := setupTopology(t)

	n := NewNode("uuid012345", "10.0.0.1")
	topo.TaskChan <- &TopoTask{
		Mode:    ADDNODE,
		Target:  n,
		IsFirst: true,
	}
	<-topo.ResultChan

	topo.TaskChan <- &TopoTask{
		Mode:     UPDATEDETAIL,
		UUID:     "uuid012345",
		UserName: "user",
		HostName: "host",
		Memo:     "memo",
	}
	// Sync
	topo.TaskChan <- &TopoTask{Mode:    CHECKNODE, UUIDNum: 0}
	<-topo.ResultChan

	// showDetail sends to ResultChan
	topo.TaskChan <- &TopoTask{Mode: SHOWDETAIL}
	<-topo.ResultChan // should not panic
}

func TestShowTopoDoesNotPanic(t *testing.T) {
	topo := setupTopology(t)

	parent := NewNode("parentuuid0", "10.0.0.1")
	topo.TaskChan <- &TopoTask{
		Mode:    ADDNODE,
		Target:  parent,
		IsFirst: true,
	}
	<-topo.ResultChan

	child := NewNode("childuuid00", "10.0.0.2")
	topo.TaskChan <- &TopoTask{
		Mode:       ADDNODE,
		Target:     child,
		ParentUUID: "parentuuid0",
		IsFirst:    false,
	}
	<-topo.ResultChan

	topo.TaskChan <- &TopoTask{Mode: SHOWTOPO}
	<-topo.ResultChan // should not panic
}

func TestMultipleNodesIDIncrement(t *testing.T) {
	topo := setupTopology(t)

	for i := 0; i < 5; i++ {
		uuid := "uuid" + string(rune('A'+i)) + "00000"
		n := NewNode(uuid, "10.0.0.1")
		topo.TaskChan <- &TopoTask{
			Mode:    ADDNODE,
			Target:  n,
			IsFirst: true,
		}
		result := <-topo.ResultChan
		if result.IDNum != i {
			t.Errorf("node %d: expected IDNum %d, got %d", i, i, result.IDNum)
		}
	}
}
