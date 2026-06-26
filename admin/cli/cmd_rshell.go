package cli

import (
	"fmt"

	"Shroud/admin/handler"
	"Shroud/admin/printer"
)

func (console *Console) cmdRShell(fCommand []string, uuidNum int, uuid string, route string) {
	if console.expectParams(fCommand, 2, NODE, 0) {
		return
	}

	port := fCommand[1]

	printer.Warning("\r\n[*] Asking agent to listen on port %s......", port)

	handler.LetRShell(console.mgr, route, uuid, port)

	select {
	case ok := <-console.mgr.RShellManager.ReadyChan:
		if !ok {
			printer.Fail("\r\n[*] Agent failed to listen on port %s", port)
			console.ready <- true
			return
		}
	case <-console.ctx.Done():
		console.ready <- true
		return
	}

	printer.Success("\r\n[*] Agent is listening on port %s, waiting for reverse shell...", port)
	printer.Warning("\r\n[*] (terminal paused until shell connects, Ctrl+C won't work here)\r\n")

	select {
	case seq := <-console.mgr.RShellManager.ConnChan:
		printer.Success("\r\n[*] Reverse shell connection received!\r\n")
		console.status = ""
		console.shellMode = true
		console.ready <- true
		console.handleRShellPanelCommand(route, uuid, seq)
		console.shellMode = false
		console.status = fmt.Sprintf("(node %d) >> ", uuidNum)
	case <-console.ctx.Done():
		console.ready <- true
	}
}

func (console *Console) handleRShellPanelCommand(route string, uuid string, seq uint64) {
	exitCh := make(chan struct{})

	go func() {
		<-console.mgr.ConsoleManager.Exit
		close(exitCh)
	}()

	defer func() {
		handler.SendRShellFin(route, uuid, seq)
		select {
		case <-console.mgr.ConsoleManager.Exit:
		default:
		}
	}()

	for {
		select {
		case tCommand := <-console.getCommand:
			handler.SendRShellData(route, uuid, seq, tCommand)
		case <-exitCh:
			printer.Warning("\r\n[*] Reverse shell disconnected\r\n")
			return
		case <-console.ctx.Done():
			return
		}
	}
}

func (console *Console) cmdStopRShell(fCommand []string, uuidNum int, uuid string, route string) {
	if console.expectParams(fCommand, 1, NODE, 0) {
		return
	}

	handler.StopRShell(console.mgr, route, uuid)

	select {
	case <-console.mgr.RShellManager.StopDoneChan:
		printer.Success("\r\n[*] Reverse shell listener stopped")
	case <-console.ctx.Done():
	}

	console.ready <- true
}
