package process

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"Shroud/admin/cli"
	"Shroud/admin/handler"
	"Shroud/admin/initial"
	"Shroud/admin/manager"
	"Shroud/admin/printer"
	"Shroud/admin/topology"
	"Shroud/global"
	"Shroud/protocol"
)

type Admin struct {
	mgr      *manager.Manager
	options  *initial.Options
	topology *topology.Topology
	reconCtx *initial.ReconnectContext
}

func NewAdmin(opt *initial.Options, topo *topology.Topology, reconCtx *initial.ReconnectContext) *Admin {
	admin := new(Admin)
	admin.topology = topo
	admin.options = opt
	admin.reconCtx = reconCtx
	return admin
}

func (admin *Admin) Run(term cli.Terminal) {
	admin.mgr = manager.NewManager()
	go admin.mgr.Run()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go admin.handleMessFromDownstream(term)
	go handler.DispatchListenMess(admin.mgr, admin.topology)
	go handler.DispatchConnectMess(admin.mgr)
	go handler.DispathSocksMess(admin.mgr, admin.topology)
	go handler.DispatchForwardMess(admin.mgr)
	go handler.DispatchBackwardMess(admin.mgr, admin.topology)
	go handler.DispatchFileMess(admin.mgr)
	go handler.DispatchSSHMess(admin.mgr)
	go handler.DispatchSSHTunnelMess(admin.mgr)
	go handler.DispatchShellMess(admin.mgr)
	go handler.DispatchInfoMess(admin.mgr, admin.topology)
	go DispatchChildrenMess(admin.mgr, admin.topology)

	if admin.options != nil && admin.options.Heartbeat {
		go handler.LetHeartbeat(ctx, admin.topology)
	}

	if admin.options != nil && admin.options.AutoSocks != "" {
		go admin.autoStartSocks(ctx)
	}

	if admin.options != nil && admin.options.Daemon {
		printer.Warning("[*] Running in daemon mode\r\n")
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
		sig := <-sigCh
		printer.Warning("[*] Received %s, shutting down...\r\n", sig)
		global.AdminCleanExit()
	}

	console := cli.NewConsole()
	console.Init(ctx, term, admin.topology, admin.mgr)
	console.Run()
}

func (admin *Admin) autoStartSocks(ctx context.Context) {
	select {
	case <-admin.topology.NodeReady:
	case <-time.After(30 * time.Second):
		printer.Fail("[*] Timeout waiting for agent, auto-socks aborted\r\n")
		return
	}

	topoTask := &topology.TopoTask{
		Mode:    topology.GETUUID,
		UUIDNum: 0,
	}
	admin.topology.TaskChan <- topoTask
	topoResult := <-admin.topology.ResultChan
	uuid := topoResult.UUID

	topoTask = &topology.TopoTask{
		Mode: topology.GETROUTE,
		UUID: uuid,
	}
	admin.topology.TaskChan <- topoTask
	topoResult = <-admin.topology.ResultChan
	route := topoResult.Route

	socks := handler.NewSocks(admin.options.AutoSocks)
	printer.Warning("[*] Auto-starting SOCKS5 on %s:%s...\r\n", socks.Addr, socks.Port)
	if err := socks.LetSocks(ctx, admin.mgr, route, uuid); err != nil {
		printer.Fail("[*] Auto-socks failed: %s\r\n", err.Error())
	} else {
		printer.Success("[*] SOCKS5 started on %s:%s\r\n", socks.Addr, socks.Port)
	}
}

func (admin *Admin) handleMessFromDownstream(term cli.Terminal) {
	rMessage := protocol.NewDownMsg(global.G_Component.Conn, global.G_Component.CryptoKey, global.Session.GetLinkKey(), global.G_Component.UUID)

	for {
		header, message, err := protocol.DestructMessage(rMessage)
		if err != nil {
			select {
			case <-global.Session.TransportSwitch:
				rMessage = protocol.NewDownMsg(global.G_Component.Conn, global.G_Component.CryptoKey, global.Session.GetLinkKey(), global.G_Component.UUID)
				continue
			default:
			}

			if admin.reconCtx != nil && admin.options.Mode != initial.NORMAL_PASSIVE {
				printer.Fail("\r\n[*] Peer node seems offline, attempting reconnection...\r\n")
				newConn, newLinkKey, reconErr := initial.ActiveReconnect(admin.reconCtx)
				if reconErr == nil {
					oldConn := global.SwapGComponentConn(newConn)
					if oldConn != nil {
						oldConn.Close()
					}
					global.Session.SetLinkKey(newLinkKey)
					rMessage = protocol.NewDownMsg(global.G_Component.Conn, global.G_Component.CryptoKey, global.Session.GetLinkKey(), global.G_Component.UUID)
					continue
				}
				printer.Fail("\r\n[*] Reconnection failed: %s\r\n", reconErr.Error())
			} else {
				printer.Fail("\r\n[*] Peer node seems offline!\r\n")
			}

			if admin.reconCtx != nil && admin.reconCtx.Daemon {
				global.AdminCleanExit()
			}
			printer.Fail("[*] Press any key to exit\r\n")
			term.PollEvent()
			term.Close()
			global.AdminCleanExit()
		}

		switch header.MessageType {
		case protocol.MYINFO:
			admin.mgr.InfoManager.InfoMessChan <- message
		case protocol.SHELLRES:
			fallthrough
		case protocol.SHELLRESULT:
			fallthrough
		case protocol.SHELLEXIT:
			admin.mgr.ShellManager.ShellMessChan <- message
		case protocol.SSHRES:
			fallthrough
		case protocol.SSHRESULT:
			fallthrough
		case protocol.SSHEXIT:
			admin.mgr.SSHManager.SSHMessChan <- message
		case protocol.SSHTUNNELRES:
			admin.mgr.SSHTunnelManager.SSHTunnelMessChan <- message
		case protocol.FILESTATREQ:
			fallthrough
		case protocol.FILEDOWNRES:
			fallthrough
		case protocol.FILESTATRES:
			fallthrough
		case protocol.FILEDATA:
			fallthrough
		case protocol.FILEERR:
			admin.mgr.FileManager.FileMessChan <- message
		case protocol.SOCKSREADY:
			fallthrough
		case protocol.SOCKSTCPDATA:
			fallthrough
		case protocol.SOCKSTCPFIN:
			fallthrough
		case protocol.UDPASSSTART:
			fallthrough
		case protocol.SOCKSUDPDATA:
			admin.mgr.SocksManager.SocksMessChan <- message
		case protocol.FORWARDREADY:
			fallthrough
		case protocol.FORWARDDATA:
			fallthrough
		case protocol.FORWARDFIN:
			admin.mgr.ForwardManager.ForwardMessChan <- message
		case protocol.BACKWARDREADY:
			fallthrough
		case protocol.BACKWARDDATA:
			fallthrough
		case protocol.BACKWARDFIN:
			fallthrough
		case protocol.BACKWARDSTOPDONE:
			fallthrough
		case protocol.BACKWARDSTART:
			admin.mgr.BackwardManager.BackwardMessChan <- message
		case protocol.CHILDUUIDREQ:
			fallthrough
		case protocol.LISTENRES:
			admin.mgr.ListenManager.ListenMessChan <- message
		case protocol.CONNECTDONE:
			admin.mgr.ConnectManager.ConnectMessChan <- message
		case protocol.NODEREONLINE:
			fallthrough
		case protocol.NODEOFFLINE:
			admin.mgr.ChildrenManager.ChildrenMessChan <- message
		case protocol.TRANSPORTSWITCHRES:
			admin.mgr.TransportManager.TransportMessChan <- message
		case protocol.HEARTBEAT:
			// agent-initiated keepalive; no action needed
		case protocol.HEARTBEATACK:
			handler.HandleHeartbeatAck()
		default:
			printer.Fail("\r\n[*] Unknown Message!")
		}
	}
}
