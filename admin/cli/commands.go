package cli

import (
	"strings"

	"Shroud/admin/printer"
	"Shroud/utils"
)

type command struct {
	name    string
	handler func(*Console, []string, int, string, string)
}

var nodeCommands = map[string]command{
	"socks":        {name: "socks", handler: (*Console).cmdSocks},
	"stopsocks":    {name: "stopsocks", handler: (*Console).cmdStopSocks},
	"forward":      {name: "forward", handler: (*Console).cmdForward},
	"stopforward":  {name: "stopforward", handler: (*Console).cmdStopForward},
	"backward":     {name: "backward", handler: (*Console).cmdBackward},
	"stopbackward": {name: "stopbackward", handler: (*Console).cmdStopBackward},
	"connect":      {name: "connect", handler: (*Console).cmdConnect},
	"listen":       {name: "listen", handler: (*Console).cmdListen},
	"transport":    {name: "transport", handler: (*Console).cmdTransport},
	"newcircuit":   {name: "newcircuit", handler: (*Console).cmdNewCircuit},
	"upload":       {name: "upload", handler: (*Console).cmdUpload},
	"download":     {name: "download", handler: (*Console).cmdDownload},
	"shell":        {name: "shell", handler: (*Console).cmdShell},
	"ssh":          {name: "ssh", handler: (*Console).cmdSSH},
	"sshtunnel":    {name: "sshtunnel", handler: (*Console).cmdSSHTunnel},
	"status":       {name: "status", handler: (*Console).cmdStatus},
	"addmemo":      {name: "addmemo", handler: (*Console).cmdAddMemo},
	"delmemo":      {name: "delmemo", handler: (*Console).cmdDelMemo},
	"shutdown":     {name: "shutdown", handler: (*Console).cmdShutdown},
	"revoke":       {name: "revoke", handler: (*Console).cmdRevoke},
	"rshell":       {name: "rshell", handler: (*Console).cmdRShell},
	"stoprshell":   {name: "stoprshell", handler: (*Console).cmdStopRShell},
}

func (console *Console) expectParams(params []string, numbers interface{}, mode int, needToBeInt interface{}) bool {
	switch nums := numbers.(type) {
	case int:
		if len(params) != nums {
			printer.Fail("\r\n[*] Format error!\r\n")
			if mode == MAIN {
				ShowMainHelp()
			} else {
				ShowNodeHelp()
			}
			console.ready <- true
			return true
		}
	case []int:
		var flag bool
		for _, num := range nums {
			if len(params) == num {
				flag = true
			}
		}

		if !flag {
			printer.Fail("\r\n[*] Format error!\r\n")
			if mode == MAIN {
				ShowMainHelp()
			} else {
				ShowNodeHelp()
			}
			console.ready <- true
			return true
		}
	}

	switch seqs := needToBeInt.(type) {
	case int:
		if needToBeInt != 0 {
			_, err := utils.Str2Int(params[seqs])
			if err != nil {
				printer.Fail("\r\n[*] Format error!\r\n")
				if mode == MAIN {
					ShowMainHelp()
				} else {
					ShowNodeHelp()
				}
				console.ready <- true
				return true
			}
		}
	case []int:
		var err error
		for _, seq := range seqs {
			if seq != 0 {
				_, err = utils.Str2Int(params[seq])
				if err != nil {
					break
				}
			}
		}

		if err != nil {
			printer.Fail("\r\n[*] Format error!\r\n")
			if mode == MAIN {
				ShowMainHelp()
			} else {
				ShowNodeHelp()
			}
			console.ready <- true
			return true
		}
	}

	return false
}

func (console *Console) pretreatInput() string {
	tCommand := <-console.getCommand
	tCommand = strings.TrimRight(tCommand, " \t\r\n")
	return tCommand
}
