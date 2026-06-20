package handler

import (
	"encoding/json"

	"Shroud/admin/topology"
	"Shroud/global"
	"Shroud/protocol"
)

func DistributeRouteTables(topo *topology.Topology) {
	topoTask := &topology.TopoTask{Mode: topology.GETROUTETABLE}
	topo.TaskChan <- topoTask
	result := <-topo.ResultChan

	if result.RouteTables == nil {
		return
	}

	for agentUUID, table := range result.RouteTables {
		if len(table) == 0 {
			continue
		}

		topoTask := &topology.TopoTask{
			Mode: topology.GETROUTE,
			UUID: agentUUID,
		}
		topo.TaskChan <- topoTask
		routeResult := <-topo.ResultChan
		firstHop := routeResult.Route

		entriesJSON, err := json.Marshal(table)
		if err != nil {
			continue
		}

		sMessage := protocol.NewDownMsg(global.G_Component.Conn, global.G_Component.CryptoKey, global.Session.LinkKey, global.G_Component.UUID)

		header := &protocol.Header{
			Sender:      protocol.ADMIN_UUID,
			Accepter:    agentUUID,
			MessageType: protocol.ROUTETABLE,
			RouteLen:    uint32(len(firstHop)),
			Route:       firstHop,
		}

		msg := &protocol.RouteTableMsg{
			EntriesLen: uint32(len(entriesJSON)),
			Entries:    string(entriesJSON),
		}

		protocol.ConstructMessage(sMessage, header, msg, false)
		sMessage.SendMessage()
	}
}
