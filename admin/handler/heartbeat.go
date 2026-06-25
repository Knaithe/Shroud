package handler

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"log"
	"sync/atomic"
	"time"

	"Shroud/admin/topology"
	"Shroud/global"
	"Shroud/protocol"
)

type HeartbeatState struct {
	seq        uint64
	missedAcks int32
}

func NewHeartbeatState() *HeartbeatState {
	return &HeartbeatState{}
}

func (hb *HeartbeatState) ResetMissed() {
	atomic.StoreInt32(&hb.missedAcks, 0)
}

func LetHeartbeat(ctx context.Context, topo *topology.Topology, hb *HeartbeatState) {
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
		select {
		case <-ctx.Done():
			return
		case <-time.After(10*time.Second + time.Duration(cryptoRandIntn(6000))*time.Millisecond):
		}

		seq := atomic.AddUint64(&hb.seq, 1)

		sMessage := protocol.NewDownMsg(global.G_Component.Conn, global.G_Component.CryptoKey, global.Session.GetLinkKey(), global.G_Component.UUID)

		header := &protocol.Header{
			Sender:      protocol.ADMIN_UUID,
			Accepter:    uuid,
			MessageType: protocol.HEARTBEAT,
			RouteLen:    uint32(len([]byte(route))),
			Route:       route,
		}

		HBMess := &protocol.HeartbeatMsg{
			Ping: 1,
			Seq:  seq,
		}

		protocol.ConstructMessage(sMessage, header, HBMess, false)
		sMessage.SendMessage()

		atomic.AddInt32(&hb.missedAcks, 1)
		if missed := atomic.LoadInt32(&hb.missedAcks); missed > 5 {
			log.Printf("[*] Node %s: %d heartbeats without ACK, closing connection", uuid, missed)
			global.G_Component.Conn.Close()
			return
		} else if missed > 3 {
			log.Printf("[*] Node %s: %d heartbeats without ACK", uuid, missed)
		}
	}
}

func cryptoRandIntn(max int) int {
	var buf [8]byte
	rand.Read(buf[:])
	return int(binary.BigEndian.Uint64(buf[:]) % uint64(max))
}
