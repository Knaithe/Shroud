package handler

import (
	"Shroud/admin/manager"
	"Shroud/global"
	"fmt"
)

func ShowStatus(mgr *manager.Manager, uuid string) {
	fmt.Print("\r\nTransport mode: ")
	fmt.Print(global.GetTransportMode())
	fmt.Print("\r\n-------------------------------------------------------------------------------------------")
	forwardInfo, forwardOK := mgr.ForwardManager.GetForwardInfo(uuid)
	backwardInfo, backwardOK := mgr.BackwardManager.GetBackwardInfo(uuid)
	socksInfo, socksOK := mgr.SocksManager.GetSocksInfo(uuid)

	fmt.Print("\r\nSocks status:")
	if socksOK {
		fmt.Printf(
			"\r\n      ListenAddr: %s:%s    Username: %s   Password: %s",
			socksInfo.Addr,
			socksInfo.Port,
			socksInfo.Username,
			socksInfo.Password,
		)
	}
	fmt.Print("\r\n-------------------------------------------------------------------------------------------")
	fmt.Print("\r\nForward status:")
	if forwardOK {
		for _, info := range forwardInfo {
			fmt.Printf(
				"\r\n      [%d] Listening Addr: %s , Remote Addr: %s , Active Connections: %d",
				info.Seq,
				info.Laddr,
				info.Raddr,
				info.ActiveNum,
			)
		}
	}
	fmt.Print("\r\n-------------------------------------------------------------------------------------------")
	fmt.Print("\r\nBackward status:")
	if backwardOK {
		for _, info := range backwardInfo {
			fmt.Printf(
				"\r\n      [%d] Remote Port: %s , Local Port: %s , Active Connections: %d",
				info.Seq,
				info.RPort,
				info.LPort,
				info.ActiveNum,
			)
		}
	}
}
