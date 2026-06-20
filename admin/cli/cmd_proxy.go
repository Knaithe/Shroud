package cli

import (
	"fmt"

	"Shroud/admin/handler"
	"Shroud/admin/printer"
	"Shroud/utils"
)

func (console *Console) cmdSocks(fCommand []string, uuidNum int, uuid string, route string) {
	if console.expectParams(fCommand, []int{2, 4}, NODE, 0) {
		return
	}

	socks := handler.NewSocks(fCommand[1])
	if len(fCommand) > 2 {
		socks.Username = fCommand[2]
		socks.Password = fCommand[3]
	}

	printer.Warning("\r\n[*] Trying to listen on %s:%s......", socks.Addr, socks.Port)

	printer.Warning("\r\n[*] Waiting for agent's response......")

	err := socks.LetSocks(console.ctx, console.mgr, route, uuid)

	if err != nil {
		printer.Fail("\r\n[*] Error: %s", err.Error())
	} else {
		printer.Success("\r\n[*] Socks start successfully!")
	}
	console.ready <- true
}

func (console *Console) cmdStopSocks(fCommand []string, uuidNum int, uuid string, route string) {
	if console.expectParams(fCommand, 1, NODE, 0) {
		return
	}

	IsRunning := handler.GetSocksInfo(console.mgr, uuid)

	if IsRunning {
		console.status = "[*] Do you really want to shut down socks?(y/n): "
		console.ready <- true
		option := console.pretreatInput()
		if option == "y" {
			printer.Warning("\r\n[*] Closing......")
			handler.StopSocks(console.mgr, uuid)
			printer.Success("\r\n[*] Socks service has been closed successfully!")
		} else if option == "n" {
		} else {
			printer.Fail("\r\n[*] Please input y/n!")
		}
		console.status = fmt.Sprintf("(node %d) >> ", uuidNum)
	} else {
		printer.Fail("\r\n[*] Socks service isn't running!")
	}
	console.ready <- true
}

func (console *Console) cmdForward(fCommand []string, uuidNum int, uuid string, route string) {
	if console.expectParams(fCommand, 3, NODE, 1) {
		return
	}

	printer.Warning("\r\n[*] Trying to listen on 127.0.0.1:%s......", fCommand[1])
	printer.Warning("\r\n[*] Waiting for agent's response......")

	forward := handler.NewForward(fCommand[1], fCommand[2])

	err := forward.LetForward(console.ctx, console.mgr, route, uuid)
	if err != nil {
		printer.Fail("\r\n[*] Error: %s", err.Error())
	} else {
		printer.Success("\r\n[*] Forward start successfully!")
	}
	console.ready <- true
}

func (console *Console) cmdStopForward(fCommand []string, uuidNum int, uuid string, route string) {
	if console.expectParams(fCommand, 1, NODE, 0) {
		return
	}

	seq, isRunning := handler.GetForwardInfo(console.mgr, uuid)

	if isRunning {
		console.status = "[*] Do you really want to shut down forward?(y/n): "
		console.ready <- true
		option := console.pretreatInput()
		if option == "y" {
			console.status = "[*] Please choose one to close: "
			console.ready <- true
			option := console.pretreatInput()
			choice, err := utils.Str2Int(option)
			if err != nil {
				printer.Fail("\r\n[*] Please input integer!")
			} else if choice > seq || choice < 0 {
				printer.Fail("\r\n[*] Please input integer between 0~%d", seq)
			} else {
				printer.Warning("\r\n[*] Closing......")
				handler.StopForward(console.mgr, uuid, choice)
				printer.Success("\r\n[*] Forward service has been closed successfully!")
			}
		} else if option == "n" {
		} else {
			printer.Fail("\r\n[*] Please input y/n!")
		}
		console.status = fmt.Sprintf("(node %d) >> ", uuidNum)
	} else {
		printer.Fail("\r\n[*] Forward service isn't running!")
	}
	console.ready <- true
}

func (console *Console) cmdBackward(fCommand []string, uuidNum int, uuid string, route string) {
	if console.expectParams(fCommand, 3, NODE, []int{1, 2}) {
		return
	}

	printer.Warning("\r\n[*] Trying to ask node to listen on 127.0.0.1:%s......", fCommand[1])
	printer.Warning("\r\n[*] Waiting for agent's response......")

	backward := handler.NewBackward(fCommand[2], fCommand[1])
	err := backward.LetBackward(console.mgr, route, uuid)
	if err != nil {
		printer.Fail("\r\n[*] Error: %s", err.Error())
	} else {
		printer.Success("\r\n[*] Backward start successfully!")
	}
	console.ready <- true
}

func (console *Console) cmdStopBackward(fCommand []string, uuidNum int, uuid string, route string) {
	if console.expectParams(fCommand, 1, NODE, 0) {
		return
	}

	seq, isRunning := handler.GetBackwardInfo(console.mgr, uuid)

	if isRunning {
		console.status = "[*] Do you really want to shut down backward?(y/n): "
		console.ready <- true
		option := console.pretreatInput()
		if option == "y" {
			console.status = "[*] Please choose one to close: "
			console.ready <- true
			option := console.pretreatInput()
			choice, err := utils.Str2Int(option)
			if err != nil {
				printer.Fail("\r\n[*] Please input integer!")
			} else if choice > seq || choice < 0 {
				printer.Fail("\r\n[*] Please input integer between 0~%d", seq)
			} else {
				printer.Warning("\r\n[*] Closing......")
				handler.StopBackward(console.mgr, uuid, route, choice)
				printer.Success("\r\n[*] Backward service has been closed successfully!")
			}
		} else if option == "n" {
		} else {
			printer.Fail("\r\n[*] Please input y/n!")
		}
		console.status = fmt.Sprintf("(node %d) >> ", uuidNum)
	} else {
		printer.Fail("\r\n[*] Backward service isn't running!")
	}
	console.ready <- true
}
