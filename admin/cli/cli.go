package cli

import (
	"fmt"

	"Shroud/protocol"
)

// Banner 程序图标
func Banner() {
	fmt.Printf(`
    _____ __                        __
   / ___// /_  _________  __  ____/ /
   \__ \/ __ \/ ___/ __ \/ / / / __/
  ___/ / / / / /  / /_/ / /_/ / /_/
 /____/_/ /_/_/   \____/\__,_/\__/
			            { %s }
`, protocol.SHROUD_VERSION)
}

// ShowMainHelp 打印admin模式下的帮助
func ShowMainHelp() {
	fmt.Print(`
	help                                     		Show help information
	detail                                  		Display connected nodes' detail
	topo                                     		Display nodes' topology
	use        <id>                          		Select the target node you want to use
	resettoken                               		Clear all consumed enrollment tokens
	exit                                     		Exit Shroud
  `)
}

// ShowNodeHelp 打印node模式下的帮助
func ShowNodeHelp() {
	fmt.Print(`
	help                                            Show help information
	status                                          Show node status,including socks/forward/backward
	listen                                          Start port listening on current node
	addmemo    <string>                             Add memo for current node
	delmemo                                         Delete memo of current node
	ssh        <ip:port>                            Start SSH through current node
	shell                                           Start an interactive shell on current node
	socks      <lport> [username] [pass]            Start a socks5 server
	stopsocks                                       Shut down socks services
	connect    <ip:port>                            Connect to a new node
	sshtunnel  <ip:sshport> <agent port>            Use sshtunnel to add the node into our topology
	upload     <local filename> <remote filename>   Upload file to current node
	download   <remote filename> <local filename>   Download file from current node
	forward    <lport> <ip:port>                    Forward local port to specific remote ip:port
	stopforward                                     Shut down forward services
	backward    <rport> <lport>                     Backward remote port(agent) to local port(admin)
	stopbackward                                    Shut down backward services
	transport  [tor|raw]                            Show/switch transport mode (Node 0 only)
	newcircuit                                      Request a new Tor circuit
	revoke                                          Revoke node certificate and shutdown
	shutdown                                        Terminate current node
	rshell     <port>                               Listen on agent port for reverse shell, interact via tunnel
	stoprshell                                      Stop reverse shell listener on agent
	back                                            Back to parent panel
	exit                                            Exit Shroud
  `)
}
