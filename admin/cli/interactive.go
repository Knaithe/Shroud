package cli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"Shroud/admin/manager"
	"Shroud/admin/printer"
	"Shroud/admin/topology"
	"Shroud/global"
	"Shroud/protocol"
	"Shroud/utils"
)

const (
	MAIN = iota
	NODE
)

type Console struct {
	// Admin status
	topology *topology.Topology
	// console internal elements
	ctx        context.Context
	term       Terminal
	status     string
	ready      chan bool
	getCommand chan string
	shellMode  bool
	sshMode    bool
	nodeMode   bool
	// manager that needs to be shared with main thread
	mgr *manager.Manager
}

func NewConsole() *Console {
	console := new(Console)
	console.status = "(admin) >> "
	console.ready = make(chan bool)
	console.getCommand = make(chan string)
	return console
}

func (console *Console) Init(ctx context.Context, term Terminal, tTopology *topology.Topology, myManager *manager.Manager) {
	console.ctx = ctx
	console.term = term
	console.topology = tTopology
	console.mgr = myManager
}

func (console *Console) Run() {
	go console.handleMainPanelCommand()
	console.start()
}

func (console *Console) start() {
	if lt, ok := console.term.(LineTerminal); ok {
		console.startLineMode(lt)
		return
	}
	console.startCharMode()
}

func (console *Console) startLineMode(lt LineTerminal) {
	for {
		line, err := lt.ReadLine()
		if err != nil {
			if err == io.EOF {
				global.AdminCleanExit()
				return
			}
			continue
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		console.getCommand <- line
		<-console.ready
	}
}

func (console *Console) startCharMode() {
	var (
		isGoingOn    bool
		leftCommand  string
		rightCommand string
	)
	// start history
	history := NewHistory()
	go history.Run()
	// start helper
	helper := NewHelper()
	go helper.Run()

	fmt.Print(console.status)

	for {
		event := console.term.PollEvent()
		if event.Err != nil {
			continue
		}

		if event.Key == KeyBackspace {
			if !console.shellMode && !console.sshMode {
				fmt.Print("\r\033[K")
				fmt.Print(console.status)

				if len(leftCommand) >= 1 {
					leftCommand = string([]rune(leftCommand)[:len([]rune(leftCommand))-1])
				}

				fmt.Print(leftCommand + rightCommand)

				notSingleNum := (len(rightCommand) - len([]rune(rightCommand))) / 2
				singleNum := len([]rune(rightCommand)) - notSingleNum
				fmt.Print(string(bytes.Repeat([]byte("\b"), notSingleNum*2+singleNum)))
			} else {
				notSingleNum := (len(leftCommand) - len([]rune(leftCommand))) / 2
				singleNum := len([]rune(leftCommand)) - notSingleNum
				fmt.Print(string(bytes.Repeat([]byte("\b"), notSingleNum*2+singleNum)))
				fmt.Print("\033[K")

				if len(leftCommand) >= 1 {
					leftCommand = string([]rune(leftCommand)[:len([]rune(leftCommand))-1])
				}

				fmt.Print(leftCommand + rightCommand)

				notSingleNum = (len(rightCommand) - len([]rune(rightCommand))) / 2
				singleNum = len([]rune(rightCommand)) - notSingleNum
				fmt.Print(string(bytes.Repeat([]byte("\b"), notSingleNum*2+singleNum)))
			}
		} else if event.Key == KeyEnter {
			if !console.shellMode && !console.sshMode {
				command := leftCommand + rightCommand
				if command != "" {
					task := &HistoryTask{
						Mode:    RECORD,
						Type:    NORMAL,
						Command: command,
					}
					history.TaskChan <- task
				}
				console.getCommand <- command
				isGoingOn = false
				leftCommand = ""
				rightCommand = ""
				<-console.ready
				fmt.Print("\r\n")
				fmt.Print(console.status)
			} else {
				fmt.Print("\r\n")

				command := leftCommand + rightCommand
				console.getCommand <- command + "\n"

				if leftCommand != "" {
					var task = &HistoryTask{
						Mode:    RECORD,
						Command: command,
					}

					if console.shellMode {
						task.Type = SHELL
					} else {
						task.Type = SSH
					}

					history.TaskChan <- task
				}

				isGoingOn = false
				leftCommand = ""
				rightCommand = ""
			}
		} else if event.Key == KeyArrowUp {
			if !console.shellMode && !console.sshMode {
				fmt.Print("\r\033[K")
				fmt.Print(console.status)
				task := &HistoryTask{
					Mode:  SEARCH,
					Type:  NORMAL,
					Order: BEGIN,
				}
				if !isGoingOn {
					history.TaskChan <- task
					isGoingOn = true
				} else {
					task.Order = NEXT
					history.TaskChan <- task
				}
				result := <-history.ResultChan
				fmt.Print(result)
				leftCommand = result
				rightCommand = ""
			} else {
				task := &HistoryTask{
					Mode:  SEARCH,
					Order: BEGIN,
				}

				if console.shellMode {
					task.Type = SHELL
				} else {
					task.Type = SSH
				}

				if !isGoingOn {
					history.TaskChan <- task
					isGoingOn = true
				} else {
					task.Order = NEXT
					history.TaskChan <- task
				}

				command := <-history.ResultChan

				notSingleNum := (len(leftCommand) - len([]rune(leftCommand))) / 2
				singleNum := len([]rune(leftCommand)) - notSingleNum
				fmt.Print(string(bytes.Repeat([]byte("\b"), notSingleNum*2+singleNum)))
				fmt.Print("\033[K")
				fmt.Print(command)

				leftCommand = command
				rightCommand = ""
			}
		} else if event.Key == KeyArrowDown {
			if !console.shellMode && !console.sshMode {
				fmt.Print("\r\033[K")
				fmt.Print(console.status)
				if isGoingOn {
					task := &HistoryTask{
						Mode:  SEARCH,
						Type:  NORMAL,
						Order: PREV,
					}
					history.TaskChan <- task
					result := <-history.ResultChan

					fmt.Print(result)
					leftCommand = result
				} else {
					leftCommand = ""
				}
				rightCommand = ""
			} else {
				var command string

				task := &HistoryTask{
					Mode:  SEARCH,
					Order: PREV,
				}

				if console.shellMode {
					task.Type = SHELL
				} else {
					task.Type = SSH
				}

				if isGoingOn {
					history.TaskChan <- task
					command = <-history.ResultChan
				} else {
					command = ""
				}

				notSingleNum := (len(leftCommand) - len([]rune(leftCommand))) / 2
				singleNum := len([]rune(leftCommand)) - notSingleNum
				fmt.Print(string(bytes.Repeat([]byte("\b"), notSingleNum*2+singleNum)))
				fmt.Print("\033[K")
				fmt.Print(command)

				leftCommand = command
				rightCommand = ""
			}
		} else if event.Key == KeyArrowLeft {
			if !console.shellMode && !console.sshMode {
				fmt.Print("\r\033[K")
				fmt.Print(console.status)
				if len([]rune(leftCommand)) >= 1 {
					rightCommand = string([]rune(leftCommand)[len([]rune(leftCommand))-1]) + rightCommand
					leftCommand = string([]rune(leftCommand)[:len([]rune(leftCommand))-1])
				}
				fmt.Print(leftCommand + rightCommand)
				notSingleNum := (len(rightCommand) - len([]rune(rightCommand))) / 2
				singleNum := len([]rune(rightCommand)) - notSingleNum
				fmt.Print(string(bytes.Repeat([]byte("\b"), notSingleNum*2+singleNum)))
			} else {
				notSingleNum := (len(leftCommand) - len([]rune(leftCommand))) / 2
				singleNum := len([]rune(leftCommand)) - notSingleNum
				fmt.Print(string(bytes.Repeat([]byte("\b"), notSingleNum*2+singleNum)))
				fmt.Print("\033[K")

				if len([]rune(leftCommand)) >= 1 {
					rightCommand = string([]rune(leftCommand)[len([]rune(leftCommand))-1]) + rightCommand
					leftCommand = string([]rune(leftCommand)[:len([]rune(leftCommand))-1])
				}

				fmt.Print(leftCommand + rightCommand)

				notSingleNum = (len(rightCommand) - len([]rune(rightCommand))) / 2
				singleNum = len([]rune(rightCommand)) - notSingleNum
				fmt.Print(string(bytes.Repeat([]byte("\b"), notSingleNum*2+singleNum)))
			}
		} else if event.Key == KeyArrowRight {
			if !console.shellMode && !console.sshMode {
				fmt.Print("\r\033[K")
				fmt.Print(console.status)
				if len([]rune(rightCommand)) > 1 {
					leftCommand = leftCommand + string([]rune(rightCommand)[:1])
					rightCommand = string([]rune(rightCommand)[1:])
				} else if len([]rune(rightCommand)) == 1 {
					leftCommand = leftCommand + string([]rune(rightCommand)[:1])
					rightCommand = ""
				}

				fmt.Print(leftCommand + rightCommand)

				notSingleNum := (len(rightCommand) - len([]rune(rightCommand))) / 2
				singleNum := len([]rune(rightCommand)) - notSingleNum
				fmt.Print(string(bytes.Repeat([]byte("\b"), notSingleNum*2+singleNum)))
			} else {
				notSingleNum := (len(leftCommand) - len([]rune(leftCommand))) / 2
				singleNum := len([]rune(leftCommand)) - notSingleNum
				fmt.Print(string(bytes.Repeat([]byte("\b"), notSingleNum*2+singleNum)))
				fmt.Print("\033[K")

				if len([]rune(rightCommand)) > 1 {
					leftCommand = leftCommand + string([]rune(rightCommand)[:1])
					rightCommand = string([]rune(rightCommand)[1:])
				} else if len([]rune(rightCommand)) == 1 {
					leftCommand = leftCommand + string([]rune(rightCommand)[:1])
					rightCommand = ""
				}

				fmt.Print(leftCommand + rightCommand)

				notSingleNum = (len(rightCommand) - len([]rune(rightCommand))) / 2
				singleNum = len([]rune(rightCommand)) - notSingleNum
				fmt.Print(string(bytes.Repeat([]byte("\b"), notSingleNum*2+singleNum)))
			}
		} else if event.Key == KeyTab {
			if len(rightCommand) != 0 || console.shellMode || console.sshMode {
				continue
			}

			task := &HelperTask{
				IsNodeMode: console.nodeMode,
				Uncomplete: leftCommand,
			}
			helper.TaskChan <- task
			compelete := <-helper.ResultChan
			if len(compelete) == 1 {
				fmt.Print("\r\033[K")
				fmt.Print(console.status)
				fmt.Print(compelete[0])
				leftCommand = compelete[0]
			} else if len(compelete) > 1 {
				fmt.Print("\r\n")
				for _, command := range compelete {
					fmt.Print(command + "    ")
				}
				fmt.Print("\r\n")
				fmt.Print(console.status)
				fmt.Print(leftCommand)
			}
		} else if event.Key == KeyCtrlC {
			if !console.shellMode && !console.sshMode {
				printer.Warning("\r\n[*] Please use 'exit' to exit Shroud or use 'back' to return to parent panel")
			} else {
				printer.Warning("\r\n[*] Press 'Enter' to force quit shell/ssh mode, other keys to continue")
				event := console.term.PollEvent()
				if event.Key == KeyEnter {
					console.mgr.ConsoleManager.Exit <- true
					printer.Success("\r\n[*] Quit shell/ssh mode successfully, press 'Enter' to continue")
				} else {
					printer.Warning("\r\n[*] Continue shell/ssh mode, press 'Enter' to continue")
				}
			}
		} else {
			if !console.shellMode && !console.sshMode {
				fmt.Print("\r\033[K")
				fmt.Print(console.status)
				if event.Key == KeySpace {
					leftCommand = leftCommand + " "
				} else {
					leftCommand = leftCommand + string(event.Char)
				}
				fmt.Print(leftCommand + rightCommand)

				notSingleNum := (len(rightCommand) - len([]rune(rightCommand))) / 2
				singleNum := len([]rune(rightCommand)) - notSingleNum
				fmt.Print(string(bytes.Repeat([]byte("\b"), notSingleNum*2+singleNum)))
			} else {
				notSingleNum := (len(leftCommand) - len([]rune(leftCommand))) / 2
				singleNum := len([]rune(leftCommand)) - notSingleNum
				fmt.Print(string(bytes.Repeat([]byte("\b"), notSingleNum*2+singleNum)))
				fmt.Print("\033[K")

				if event.Key == KeySpace {
					leftCommand = leftCommand + " "
				} else {
					leftCommand = leftCommand + string(event.Char)
				}

				fmt.Print(leftCommand + rightCommand)

				notSingleNum = (len(rightCommand) - len([]rune(rightCommand))) / 2
				singleNum = len([]rune(rightCommand)) - notSingleNum
				fmt.Print(string(bytes.Repeat([]byte("\b"), notSingleNum*2+singleNum)))
			}
		}
	}
}

// handle ur command
func (console *Console) handleMainPanelCommand() {
	for {
		tCommand := console.pretreatInput()

		var fCommand []string
		for _, command := range strings.Split(tCommand, " ") {
			if command != "" {
				fCommand = append(fCommand, command)
			}
		}

		if len(fCommand) == 0 {
			fCommand = append(fCommand, "")
		}

		switch fCommand[0] {
		case "use":
			if console.expectParams(fCommand, 2, MAIN, 1) {
				break
			}

			uuidNum, _ := utils.Str2Int(fCommand[1])

			if console.isOnline(uuidNum) {
				console.nodeMode = true
				console.status = fmt.Sprintf("(node %s) >> ", fCommand[1])
				console.handleNodePanelCommand(uuidNum)
				console.status = "(admin) >> "
				console.nodeMode = false
			} else {
				printer.Fail("\r\n[*] Node %s doesn't exist!", fCommand[1])
			}

			console.ready <- true
		case "detail":
			if console.expectParams(fCommand, 1, MAIN, 0) {
				break
			}

			task := &topology.TopoTask{
				Mode: topology.SHOWDETAIL,
			}

			console.topology.TaskChan <- task
			<-console.topology.ResultChan

			console.ready <- true
		case "topo":
			if console.expectParams(fCommand, 1, MAIN, 0) {
				break
			}

			task := &topology.TopoTask{
				Mode: topology.SHOWTOPO,
			}
			console.topology.TaskChan <- task
			<-console.topology.ResultChan

			console.ready <- true
		case "":
			if console.expectParams(fCommand, 1, MAIN, 0) {
				break
			}
			console.ready <- true
		case "help":
			if console.expectParams(fCommand, 1, MAIN, 0) {
				break
			}

			ShowMainHelp()

			console.ready <- true
		case "exit":
			if console.expectParams(fCommand, 1, MAIN, 0) {
				break
			}

			console.status = "[*] Do you really want to exit Shroud? (y/n): "
			console.ready <- true
			option := console.pretreatInput()

			if option == "y" {
				printer.Warning("\r\n[*] BYE!")
				console.term.Close()
				global.AdminCleanExit()
			}

			console.status = "(admin) >> "
			console.ready <- true
		default:
			printer.Fail("\r\n[*] Unknown Command!\r\n")
			ShowMainHelp()
			console.ready <- true
		}
	}
}

func (console *Console) handleNodePanelCommand(uuidNum int) {
	topoTask := &topology.TopoTask{
		Mode:    topology.GETUUID,
		UUIDNum: uuidNum,
	}
	console.topology.TaskChan <- topoTask
	topoResult := <-console.topology.ResultChan
	uuid := topoResult.UUID

	topoTask = &topology.TopoTask{
		Mode: topology.GETROUTE,
		UUID: uuid,
	}
	console.topology.TaskChan <- topoTask
	topoResult = <-console.topology.ResultChan
	route := topoResult.Route

	console.ready <- true

	for {
		tCommand := console.pretreatInput()
		fCommand := strings.Split(tCommand, " ")

		// Check if node is still online for commands that need it
		switch fCommand[0] {
		case "", "help", "back", "exit":
			// These don't need online check
		default:
			if !console.isOnline(uuidNum) {
				return
			}
		}

		// Look up command in registry
		if cmd, ok := nodeCommands[fCommand[0]]; ok {
			cmd.handler(console, fCommand, uuidNum, uuid, route)
			continue
		}

		// Handle built-in node panel commands
		switch fCommand[0] {
		case "":
			if console.expectParams(fCommand, 1, NODE, 0) {
				break
			}
			console.ready <- true
		case "help":
			if console.expectParams(fCommand, 1, NODE, 0) {
				break
			}

			ShowNodeHelp()
			console.ready <- true
		case "back":
			if console.expectParams(fCommand, 1, NODE, 0) {
				break
			}
			return
		case "exit":
			if console.expectParams(fCommand, 1, NODE, 0) {
				break
			}

			console.status = "[*] Do you really want to exit Shroud? (y/n): "
			console.ready <- true
			option := console.pretreatInput()

			if option == "y" {
				printer.Warning("\r\n[*] BYE!")
				console.term.Close()
				global.AdminCleanExit()
			}

			console.status = fmt.Sprintf("(node %d) >> ", uuidNum)
			console.ready <- true
		default:
			printer.Fail("\r\n[*] Unknown Command!\r\n")
			ShowNodeHelp()
			console.ready <- true
		}
	}
}

func (console *Console) handleShellPanelCommand(route string, uuid string) {
	sMessage := protocol.NewDownMsg(global.G_Component.Conn, global.G_Component.CryptoKey, global.Session.GetLinkKey(), global.G_Component.UUID)

	header := &protocol.Header{
		Sender:      protocol.ADMIN_UUID,
		Accepter:    uuid,
		MessageType: protocol.SHELLCOMMAND,
		RouteLen:    uint32(len([]byte(route))),
		Route:       route,
	}

	console.ready <- true

	for {
		select {
		case tCommand := <-console.getCommand:
			shellCommandMess := &protocol.ShellCommand{
				CommandLen: uint64(len(tCommand)),
				Command:    tCommand,
			}
			protocol.ConstructMessage(sMessage, header, shellCommandMess, false)
			sMessage.SendMessage()
		case <-console.mgr.ConsoleManager.Exit:
			return
		}
	}
}

func (console *Console) handleSSHPanelCommand(route string, uuid string) {
	sMessage := protocol.NewDownMsg(global.G_Component.Conn, global.G_Component.CryptoKey, global.Session.GetLinkKey(), global.G_Component.UUID)

	header := &protocol.Header{
		Sender:      protocol.ADMIN_UUID,
		Accepter:    uuid,
		MessageType: protocol.SSHCOMMAND,
		RouteLen:    uint32(len([]byte(route))),
		Route:       route,
	}

	console.ready <- true

	for {
		select {
		case tCommand := <-console.getCommand:
			sshCommandMess := &protocol.SSHCommand{
				CommandLen: uint64(len(tCommand)),
				Command:    tCommand,
			}
			protocol.ConstructMessage(sMessage, header, sshCommandMess, false)
			sMessage.SendMessage()
		case <-console.mgr.ConsoleManager.Exit:
			return
		}
	}
}

func (console *Console) isOnline(uuidNum int) bool {
	task := &topology.TopoTask{
		Mode:    topology.CHECKNODE,
		UUIDNum: uuidNum,
	}
	console.topology.TaskChan <- task

	result := <-console.topology.ResultChan
	if result.IsExist {
		return true
	}

	printer.Fail("\r\n[*] Node %d seems offline!", uuidNum)
	return false
}
