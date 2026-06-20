package topology

import (
	"fmt"

	"Shroud/admin/printer"
	"Shroud/protocol"
	"Shroud/utils"
)

const (
	// Topology
	ADDNODE = iota
	GETUUID
	GETUUIDNUM
	CHECKNODE
	CALCULATE
	GETROUTE
	DELNODE
	REONLINENODE
	GETROUTETABLE
	// User-friendly
	UPDATEDETAIL
	SHOWDETAIL
	SHOWTOPO
	UPDATEMEMO
)

// IDNum is only for user-friendly,uuid is used internally
type Topology struct {
	currentIDNum int
	nodes        map[int]*node     // we use uuidNum as the map's key,that's the only special exception
	route        map[string]string // map[uuid]firstHopChildUUID
	routeTables  map[string]map[string]string // agentUUID -> (destUUID -> nextHopChildUUID)
	history      map[string]int

	TaskChan   chan *TopoTask
	ResultChan chan *topoResult
}

type node struct {
	uuid            string
	parentUUID      string
	childrenUUID    []string
	currentUser     string
	currentHostname string
	currentIP       string
	memo            string
	transportMode   string
}

type TopoTask struct {
	Mode       int
	UUID       string
	UUIDNum    int
	ParentUUID string
	Target     *node
	HostName   string
	UserName   string
	Memo       string
	IsFirst    bool
}

type topoResult struct {
	IsExist     bool
	UUID        string
	Route       string
	IDNum       int
	AllNodes    []string
	RouteTables map[string]map[string]string
}

func NewTopology() *Topology {
	topology := new(Topology)
	topology.nodes = make(map[int]*node)
	topology.route = make(map[string]string)
	topology.history = make(map[string]int)
	topology.currentIDNum = 0
	topology.TaskChan = make(chan *TopoTask)
	topology.ResultChan = make(chan *topoResult)
	return topology
}

func NewNode(uuid string, ip string) *node {
	node := new(node)
	node.uuid = uuid
	node.currentIP = ip
	return node
}

func (topology *Topology) Run() {
	for {
		task := <-topology.TaskChan
		switch task.Mode {
		case ADDNODE:
			topology.addNode(task)
		case GETUUID:
			topology.getUUID(task)
		case GETUUIDNUM:
			topology.getUUIDNum(task)
		case CHECKNODE:
			topology.checkNode(task)
		case UPDATEDETAIL:
			topology.updateDetail(task)
		case SHOWDETAIL:
			topology.showDetail()
		case SHOWTOPO:
			topology.showTopo()
		case UPDATEMEMO:
			topology.updateMemo(task)
		case CALCULATE:
			topology.calculate()
		case GETROUTE:
			topology.getRoute(task)
		case DELNODE:
			topology.delNode(task)
		case REONLINENODE:
			topology.reonlineNode(task)
		case GETROUTETABLE:
			topology.getRouteTables()
		}
	}
}

func (topology *Topology) id2IDNum(uuid string) int {
	for idNum, tNode := range topology.nodes {
		if tNode.uuid == uuid {
			return idNum
		}
	}
	return -1
}

func (topology *Topology) idNum2ID(uuidNum int) string {
	return topology.nodes[uuidNum].uuid
}

func (topology *Topology) getUUID(task *TopoTask) {
	topology.ResultChan <- &topoResult{UUID: topology.idNum2ID(task.UUIDNum)}
}

func (topology *Topology) getUUIDNum(task *TopoTask) {
	topology.ResultChan <- &topoResult{IDNum: topology.id2IDNum(task.UUID)}
}

func (topology *Topology) checkNode(task *TopoTask) {
	if _, ok := topology.nodes[task.UUIDNum]; ok {
		topology.ResultChan <- &topoResult{IsExist: true}
	} else {
		topology.ResultChan <- &topoResult{IsExist: false}
	}
}

func (topology *Topology) addNode(task *TopoTask) {
	if task.IsFirst {
		task.Target.parentUUID = protocol.ADMIN_UUID
	} else {
		task.Target.parentUUID = task.ParentUUID
		parentIDNum := topology.id2IDNum(task.ParentUUID)
		if parentIDNum >= 0 {
			topology.nodes[parentIDNum].childrenUUID = append(topology.nodes[parentIDNum].childrenUUID, task.Target.uuid)
		} else {
			return
		}
	}

	topology.nodes[topology.currentIDNum] = task.Target

	topology.history[task.Target.uuid] = topology.currentIDNum

	topology.ResultChan <- &topoResult{IDNum: topology.currentIDNum}

	topology.currentIDNum++
}

func (topology *Topology) calculate() {
	newRouteInfo := make(map[string]string)

	for currentIDNum := range topology.nodes {
		currentID := topology.nodes[currentIDNum].uuid

		if topology.nodes[currentIDNum].parentUUID == protocol.ADMIN_UUID {
			newRouteInfo[currentID] = currentID
			continue
		}

		tempIDNum := currentIDNum
		for {
			if topology.nodes[tempIDNum].parentUUID == protocol.ADMIN_UUID {
				newRouteInfo[currentID] = topology.nodes[tempIDNum].uuid
				break
			}
			found := false
			for nextIDNum := range topology.nodes {
				if topology.nodes[nextIDNum].uuid == topology.nodes[tempIDNum].parentUUID {
					tempIDNum = nextIDNum
					found = true
					break
				}
			}
			if !found {
				break
			}
		}
	}

	topology.route = newRouteInfo
	topology.routeTables = topology.buildRoutingTables()

	topology.ResultChan <- &topoResult{}
}

func (topology *Topology) buildRoutingTables() map[string]map[string]string {
	tables := make(map[string]map[string]string)

	for idNum, n := range topology.nodes {
		table := make(map[string]string)
		for _, childUUID := range n.childrenUUID {
			table[childUUID] = childUUID
			childIDNum := topology.id2IDNum(childUUID)
			if childIDNum >= 0 {
				topology.addDescendantRoutes(table, childIDNum, childUUID)
			}
		}
		if len(table) > 0 {
			tables[topology.nodes[idNum].uuid] = table
		}
	}
	return tables
}

func (topology *Topology) addDescendantRoutes(table map[string]string, parentIDNum int, nextHop string) {
	if topology.nodes[parentIDNum] == nil {
		return
	}
	for _, childUUID := range topology.nodes[parentIDNum].childrenUUID {
		table[childUUID] = nextHop
		childIDNum := topology.id2IDNum(childUUID)
		if childIDNum >= 0 {
			topology.addDescendantRoutes(table, childIDNum, nextHop)
		}
	}
}

func (topology *Topology) getRouteTables() {
	topology.ResultChan <- &topoResult{RouteTables: topology.routeTables}
}

func (topology *Topology) getRoute(task *TopoTask) {
	topology.ResultChan <- &topoResult{Route: topology.route[task.UUID]}
}

func (topology *Topology) updateDetail(task *TopoTask) {
	uuidNum := topology.id2IDNum(task.UUID)
	if uuidNum >= 0 {
		topology.nodes[uuidNum].currentUser = task.UserName
		topology.nodes[uuidNum].currentHostname = task.HostName
		topology.nodes[uuidNum].memo = task.Memo
	}
}

func (topology *Topology) showDetail() {
	var nodes []int
	for uuidNum := range topology.nodes {
		nodes = append(nodes, uuidNum)
	}

	utils.CheckRange(nodes)

	for _, uuidNum := range nodes {
		transport := topology.nodes[uuidNum].transportMode
		if transport == "" {
			transport = "raw"
		}
		fmt.Printf("\r\nNode[%d] -> IP: %s  Hostname: %s  User: %s  Transport: %s\r\nMemo: %s\r\n",
			uuidNum,
			topology.nodes[uuidNum].currentIP,
			topology.nodes[uuidNum].currentHostname,
			topology.nodes[uuidNum].currentUser,
			transport,
			topology.nodes[uuidNum].memo,
		)
	}

	topology.ResultChan <- &topoResult{} // Just tell upstream: work done!
}

func (topology *Topology) showTopo() {
	var nodes []int
	for uuidNum := range topology.nodes {
		nodes = append(nodes, uuidNum)
	}

	utils.CheckRange(nodes)

	for _, uuidNum := range nodes {
		fmt.Printf("\r\nNode[%d]'s children ->\r\n", uuidNum)
		for _, child := range topology.nodes[uuidNum].childrenUUID {
			fmt.Printf("Node[%d]\r\n", topology.id2IDNum(child))
		}
	}

	topology.ResultChan <- &topoResult{} // Just tell upstream: work done!
}

func (topology *Topology) updateMemo(task *TopoTask) {
	uuidNum := topology.id2IDNum(task.UUID)
	if uuidNum >= 0 {
		topology.nodes[uuidNum].memo = task.Memo
	}
}

func (topology *Topology) delNode(task *TopoTask) {
	// find all children node,del them
	var ready []int
	var readyUUID []string

	idNum := topology.id2IDNum(task.UUID)

	parentIDNum := topology.id2IDNum(topology.nodes[idNum].parentUUID)

	for pointer, childUUID := range topology.nodes[parentIDNum].childrenUUID { // del parent's children record
		if childUUID == task.UUID {
			if pointer == len(topology.nodes[parentIDNum].childrenUUID)-1 {
				topology.nodes[parentIDNum].childrenUUID = topology.nodes[parentIDNum].childrenUUID[:pointer]
			} else {
				topology.nodes[parentIDNum].childrenUUID = append(topology.nodes[parentIDNum].childrenUUID[:pointer], topology.nodes[parentIDNum].childrenUUID[pointer+1:]...)
			}
		}
	}

	topology.findChildrenNodes(&ready, idNum)

	ready = append(ready, idNum)

	for _, idNum := range ready {
		printer.Fail("\r\n[*] Node %d is offline!", idNum)
		readyUUID = append(readyUUID, topology.idNum2ID(idNum))
		delete(topology.nodes, idNum)
	}

	topology.ResultChan <- &topoResult{AllNodes: readyUUID}
}

func (topology *Topology) findChildrenNodes(ready *[]int, idNum int) {
	for _, uuid := range topology.nodes[idNum].childrenUUID {
		idNum := topology.id2IDNum(uuid)
		*ready = append(*ready, idNum)
		topology.findChildrenNodes(ready, idNum)
	}
}

func (topology *Topology) reonlineNode(task *TopoTask) {
	if task.IsFirst {
		task.Target.parentUUID = protocol.ADMIN_UUID
	} else {
		task.Target.parentUUID = task.ParentUUID
		parentIDNum := topology.id2IDNum(task.ParentUUID)
		topology.nodes[parentIDNum].childrenUUID = append(topology.nodes[parentIDNum].childrenUUID, task.Target.uuid)
	}

	var idNum int
	if _, ok := topology.history[task.Target.uuid]; ok {
		idNum = topology.history[task.Target.uuid]
	} else {
		idNum = topology.currentIDNum
		topology.history[task.Target.uuid] = idNum
		topology.currentIDNum++
	}

	topology.nodes[idNum] = task.Target

	topology.ResultChan <- &topoResult{}
}
