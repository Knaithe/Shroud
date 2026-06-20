package process

import (
	"Shroud/admin/handler"
	"Shroud/admin/manager"
	"Shroud/admin/printer"
	"Shroud/admin/topology"
	"Shroud/protocol"
)

func nodeOffline(mgr *manager.Manager, topo *topology.Topology, uuid string) {
	topoTask := &topology.TopoTask{
		Mode: topology.DELNODE,
		UUID: uuid,
	}
	topo.TaskChan <- topoTask
	result := <-topo.ResultChan
	allNodes := result.AllNodes

	for _, nodeUUID := range allNodes {
		mgr.BackwardManager.ForceShutdown(nodeUUID)
		mgr.ForwardManager.ForceShutdown(nodeUUID)
		mgr.SocksManager.ForceShutdown(nodeUUID)
	}

	topoTask = &topology.TopoTask{
		Mode: topology.CALCULATE,
	}
	topo.TaskChan <- topoTask
	<-topo.ResultChan
	handler.DistributeRouteTables(topo)
}

func nodeReonline(mgr *manager.Manager, topo *topology.Topology, mess *protocol.NodeReonline) {
	node := topology.NewNode(mess.UUID, mess.IP)

	topoTask := &topology.TopoTask{
		Mode:       topology.REONLINENODE,
		Target:     node,
		ParentUUID: mess.ParentUUID,
		IsFirst:    false,
	}
	topo.TaskChan <- topoTask
	<-topo.ResultChan

	topoTask = &topology.TopoTask{
		Mode: topology.CALCULATE,
	}
	topo.TaskChan <- topoTask
	<-topo.ResultChan
	handler.DistributeRouteTables(topo)

	topoTask = &topology.TopoTask{
		Mode: topology.GETUUIDNUM,
		UUID: mess.UUID,
	}
	topo.TaskChan <- topoTask
	result := <-topo.ResultChan

	printer.Success("\r\n[*] Node %d is back online!", result.IDNum)
}

func DispatchChildrenMess(mgr *manager.Manager, topo *topology.Topology) {
	for {
		message := <-mgr.ChildrenManager.ChildrenMessChan

		switch mess := message.(type) {
		case *protocol.NodeOffline:
			nodeOffline(mgr, topo, mess.UUID)
		case *protocol.NodeReonline:
			nodeReonline(mgr, topo, mess)
		}
	}
}
