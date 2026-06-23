package cli

import (
	"fmt"

	"Shroud/admin/handler"
	"Shroud/admin/printer"
	"Shroud/global"
	"Shroud/share"
)

func (console *Console) cmdConnect(fCommand []string, uuidNum int, uuid string, route string) {
	if console.expectParams(fCommand, 2, NODE, 0) {
		return
	}

	printer.Warning("\r\n[*] Waiting for response......")

	err := handler.LetConnect(console.mgr, route, uuid, fCommand[1])
	if err != nil {
		printer.Fail("[*] Error: %s\n", err.Error())
	}

	console.status = fmt.Sprintf("(node %d) >> ", uuidNum)

	console.ready <- true
}

func (console *Console) cmdListen(fCommand []string, uuidNum int, uuid string, route string) {
	listen := handler.NewListen()

	if len(fCommand) >= 3 {
		listen.Method = handler.NORMAL
		listen.Addr = fCommand[2]
		if fCommand[1] == "1" {
			listen.Method = handler.NORMAL
		} else if fCommand[1] == "2" {
			listen.Method = handler.IPTABLES
		} else if fCommand[1] == "3" {
			listen.Method = handler.SOREUSE
		} else if fCommand[1] == "4" {
			listen.Method = handler.TORHIDDEN
		} else {
			printer.Fail("\r\n[*] Usage: listen <1-4> [ip:port]")
			console.status = fmt.Sprintf("(node %d) >> ", uuidNum)
			console.ready <- true
			return
		}

		printer.Warning("\r\n[*] Waiting for response......")
		err := listen.LetListen(console.mgr, route, uuid)
		if err != nil {
			printer.Fail("[*] Error: %s\n", err.Error())
		}
		console.status = fmt.Sprintf("(node %d) >> ", uuidNum)
		console.ready <- true
		return
	}

	if console.expectParams(fCommand, 1, NODE, 0) {
		return
	}

	printer.Warning("\r\n[*] BE AWARE! If you choose IPTables Reuse or SOReuse, you MUST CONFIRM that the node you're controlling was started in the corresponding way!")
	printer.Warning("\r\n[*] When you choose IPTables Reuse or SOReuse, the node will use the initial config(when node started) to reuse port!")
	console.status = "[*] Please choose the mode(1. Normal passive/2. IPTables Reuse/3. SOReuse/4. Tor Hidden Service): "
	console.ready <- true

	option := console.pretreatInput()
	if option == "1" {
		listen.Method = handler.NORMAL
		console.status = "[*] Please input the [ip:]<port> : "
		console.ready <- true
		option = console.pretreatInput()
		listen.Addr = option
	} else if option == "2" {
		listen.Method = handler.IPTABLES
	} else if option == "3" {
		listen.Method = handler.SOREUSE
	} else if option == "4" {
		listen.Method = handler.TORHIDDEN
	} else {
		printer.Fail("\r\n[*] Please input 1/2/3/4!")
		console.status = fmt.Sprintf("(node %d) >> ", uuidNum)
		console.ready <- true
		return
	}

	printer.Warning("\r\n[*] Waiting for response......")

	err := listen.LetListen(console.mgr, route, uuid)
	if err != nil {
		printer.Fail("[*] Error: %s\n", err.Error())
	}

	console.status = fmt.Sprintf("(node %d) >> ", uuidNum)

	console.ready <- true
}

func (console *Console) cmdTransport(fCommand []string, uuidNum int, uuid string, route string) {
	if uuidNum != 0 {
		printer.Fail("\r\n[*] Transport switching only works on Node 0 (direct connection)")
		console.ready <- true
		return
	}

	if len(fCommand) == 1 {
		printer.Warning("\r\n[*] Current transport mode: %s", global.GetTransportMode())
		console.ready <- true
		return
	}

	if fCommand[1] == "tor" {
		printer.Warning("\r\n[*] Switching transport to Tor...")
		console.status = "[*] Tor SOCKS5 proxy address (default 127.0.0.1:9050): "
		console.ready <- true
		torAddr := console.pretreatInput()
		if torAddr == "" {
			torAddr = "127.0.0.1:9050"
		}
		handler.SwitchTransport(console.mgr, route, uuid, handler.TRANSPORT_TOR, torAddr)
		handler.HandleTransportSwitchRes(console.mgr, handler.TRANSPORT_TOR, torAddr)
	} else if fCommand[1] == "raw" {
		printer.Warning("\r\n[*] Switching transport to raw TCP...")
		handler.SwitchTransport(console.mgr, route, uuid, handler.TRANSPORT_RAW, "")
		handler.HandleTransportSwitchRes(console.mgr, handler.TRANSPORT_RAW, "")
	} else {
		printer.Fail("\r\n[*] Usage: transport [tor|raw]")
	}
	console.status = fmt.Sprintf("(node %d) >> ", uuidNum)
	console.ready <- true
}

func (console *Console) cmdNewCircuit(fCommand []string, uuidNum int, uuid string, route string) {
	if console.expectParams(fCommand, 1, NODE, 0) {
		return
	}

	console.status = "[*] Tor control address (default 127.0.0.1:9051): "
	console.ready <- true
	controlAddr := console.pretreatInput()
	if controlAddr == "" {
		controlAddr = "127.0.0.1:9051"
	}
	console.status = "[*] Tor control password (empty for none): "
	console.ready <- true
	controlPW := console.pretreatInput()

	tc := share.NewTorControl(controlAddr, controlPW)
	if err := tc.Connect(); err != nil {
		printer.Fail("\r\n[*] Cannot connect to Tor control: %s", err.Error())
	} else if err := tc.Authenticate(); err != nil {
		printer.Fail("\r\n[*] Auth failed: %s", err.Error())
		tc.Close()
	} else if err := tc.SignalNewnym(); err != nil {
		printer.Fail("\r\n[*] NEWNYM failed: %s", err.Error())
		tc.Close()
	} else {
		printer.Success("\r\n[*] New Tor circuit established!")
		tc.Close()
	}

	console.status = fmt.Sprintf("(node %d) >> ", uuidNum)
	console.ready <- true
}
