package cli

import (
	"fmt"

	"Shroud/admin/handler"
	"Shroud/admin/printer"
)

func (console *Console) cmdStatus(fCommand []string, uuidNum int, uuid string, route string) {
	handler.ShowStatus(console.mgr, uuid)
	console.ready <- true
}

func (console *Console) cmdAddMemo(fCommand []string, uuidNum int, uuid string, route string) {
	handler.AddMemo(console.topology.TaskChan, fCommand[1:], uuid, route)
	console.ready <- true
}

func (console *Console) cmdDelMemo(fCommand []string, uuidNum int, uuid string, route string) {
	if console.expectParams(fCommand, 1, NODE, 0) {
		return
	}

	handler.DelMemo(console.topology.TaskChan, uuid, route)
	console.ready <- true
}

func (console *Console) cmdShell(fCommand []string, uuidNum int, uuid string, route string) {
	if console.expectParams(fCommand, 1, NODE, 0) {
		return
	}

	printer.Warning("\r\n[*] Waiting for response.....")

	handler.LetShellStart(route, uuid)

	if <-console.mgr.ConsoleManager.OK {
		console.status = ""
		console.shellMode = true
		console.handleShellPanelCommand(route, uuid)
		console.shellMode = false
		console.status = fmt.Sprintf("(node %d) >> ", uuidNum)
	} else {
		printer.Fail("\r\n[*] Shell cannot be started!")
		console.ready <- true
	}
}

func (console *Console) cmdSSH(fCommand []string, uuidNum int, uuid string, route string) {
	if console.expectParams(fCommand, 2, NODE, 0) {
		return
	}

	ssh := handler.NewSSH(fCommand[1])

	console.status = "[*] Please choose the auth method(1. username/password 2. certificate): "
	console.ready <- true

	firstChoice := console.pretreatInput()
	if firstChoice == "1" {
		ssh.Method = handler.UPMETHOD
	} else if firstChoice == "2" {
		ssh.Method = handler.CERMETHOD
	} else {
		printer.Fail("\r\n[*] Please input 1 or 2!")
		console.status = fmt.Sprintf("(node %d) >> ", uuidNum)
		console.ready <- true
		return
	}

	switch ssh.Method {
	case handler.UPMETHOD:
		console.status = "[*] Please enter the username: "
		console.ready <- true
		ssh.Username = console.pretreatInput()
		console.status = "[*] Please enter the password: "
		console.ready <- true
		ssh.Password = console.pretreatInput()
	case handler.CERMETHOD:
		console.status = "[*] Please enter the username: "
		console.ready <- true
		ssh.Username = console.pretreatInput()
		console.status = "[*] Please enter the filepath of the privkey: "
		console.ready <- true
		ssh.CertificatePath = console.pretreatInput()
	}

	console.status = "[*] Enter expected SSH host key fingerprint (blank for TOFU): "
	console.ready <- true
	ssh.HostKeyFingerprint = console.pretreatInput()

	printer.Warning("\r\n[*] Waiting for response.....")

	err := ssh.LetSSH(route, uuid)
	if err != nil {
		printer.Fail("\r\n[*] Error: %s", err.Error())
		console.status = fmt.Sprintf("(node %d) >> ", uuidNum)
		console.ready <- true
		return
	}

	if <-console.mgr.ConsoleManager.OK {
		console.status = ""
		console.sshMode = true
		console.handleSSHPanelCommand(route, uuid)
		console.status = fmt.Sprintf("(node %d) >> ", uuidNum)
		console.sshMode = false
	} else {
		printer.Fail("\r\n[*] Fail to connect to target host via ssh!")
		console.status = fmt.Sprintf("(node %d) >> ", uuidNum)
		console.ready <- true
	}
}

func (console *Console) cmdSSHTunnel(fCommand []string, uuidNum int, uuid string, route string) {
	if console.expectParams(fCommand, 3, NODE, 2) {
		return
	}

	sshTunnel := handler.NewSSHTunnel(fCommand[2], fCommand[1])

	console.status = "[*] Please choose the auth method(1. username/password 2. certificate): "
	console.ready <- true

	firstChoice := console.pretreatInput()
	if firstChoice == "1" {
		sshTunnel.Method = handler.UPMETHOD
	} else if firstChoice == "2" {
		sshTunnel.Method = handler.CERMETHOD
	} else {
		printer.Fail("\r\n[*] Please input 1 or 2!")
		console.status = fmt.Sprintf("(node %d) >> ", uuidNum)
		console.ready <- true
		return
	}

	switch sshTunnel.Method {
	case handler.UPMETHOD:
		console.status = "[*] Please enter the username: "
		console.ready <- true
		sshTunnel.Username = console.pretreatInput()
		console.status = "[*] Please enter the password: "
		console.ready <- true
		sshTunnel.Password = console.pretreatInput()
	case handler.CERMETHOD:
		console.status = "[*] Please enter the username: "
		console.ready <- true
		sshTunnel.Username = console.pretreatInput()
		console.status = "[*] Please enter the filepath of the privkey: "
		console.ready <- true
		sshTunnel.CertificatePath = console.pretreatInput()
	}

	console.status = "[*] Enter expected SSH host key fingerprint (blank for TOFU): "
	console.ready <- true
	sshTunnel.HostKeyFingerprint = console.pretreatInput()

	printer.Warning("\r\n[*] Waiting for response.....")

	err := sshTunnel.LetSSHTunnel(route, uuid)
	if err != nil {
		printer.Fail("\r\n[*] Error: %s", err.Error())
		console.status = fmt.Sprintf("(node %d) >> ", uuidNum)
		console.ready <- true
		return
	}

	if ok := <-console.mgr.ConsoleManager.OK; !ok {
		printer.Fail("\r\n[*] Fail to add target node via SSHTunnel!")
	}

	console.status = fmt.Sprintf("(node %d) >> ", uuidNum)
	console.ready <- true
}

func (console *Console) cmdShutdown(fCommand []string, uuidNum int, uuid string, route string) {
	if console.expectParams(fCommand, 1, NODE, 0) {
		return
	}

	handler.LetShutdown(route, uuid)
	console.ready <- true
}
