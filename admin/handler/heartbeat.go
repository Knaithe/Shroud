package handler

import (
	"crypto/rand"
	"encoding/binary"
	"time"

	"Shroud/admin/topology"
	"Shroud/global"
	"Shroud/protocol"
)

func LetHeartbeat(topo *topology.Topology) {
	topoTask := &topology.TopoTask{
		Mode:    topology.GETUUID,
		UUIDNum: 0,
	}

	topo.TaskChan <- topoTask
	topoResult := <-topo.ResultChan
	uuid := topoResult.UUID

	topoTask = &topology.TopoTask{
		Mode: topology.GETROUTE,
		UUID: uuid,
	}
	topo.TaskChan <- topoTask
	topoResult = <-topo.ResultChan
	route := topoResult.Route

	for {
		time.Sleep(10*time.Second + time.Duration(cryptoRandIntn(6000))*time.Millisecond)

		sMessage := protocol.NewDownMsg(global.G_Component.Conn, global.G_Component.CryptoKey, global.Session.LinkKey, global.G_Component.UUID)

		header := &protocol.Header{
			Sender:      protocol.ADMIN_UUID,
			Accepter:    uuid,
			MessageType: protocol.HEARTBEAT,
			RouteLen:    uint32(len([]byte(route))),
			Route:       route,
		}

		HBMess := &protocol.HeartbeatMsg{
			Ping: 1,
		}

		protocol.ConstructMessage(sMessage, header, HBMess, false)
		sMessage.SendMessage()
	}
}

func cryptoRandIntn(max int) int {
	var buf [8]byte
	rand.Read(buf[:])
	return int(binary.BigEndian.Uint64(buf[:]) % uint64(max))
}
