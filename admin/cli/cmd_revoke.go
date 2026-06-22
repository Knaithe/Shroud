package cli

import (
	"Shroud/admin/handler"
	"Shroud/admin/printer"
	"Shroud/admin/topology"
	"Shroud/identity"
)

var adminIdentity *identity.AdminStore

func SetAdminIdentity(id *identity.AdminStore) { adminIdentity = id }

func (console *Console) cmdRevoke(fCommand []string, uuidNum int, uuid string, route string) {
	if adminIdentity == nil {
		printer.Fail("\r\n[*] Identity store not available")
		console.ready <- true
		return
	}

	topoTask := &topology.TopoTask{
		Mode: topology.CHECKNODE,
		UUID: uuid,
	}
	console.topology.TaskChan <- topoTask
	topoResult := <-console.topology.ResultChan
	if !topoResult.IsExist {
		printer.Fail("\r\n[*] Node not found")
		console.ready <- true
		return
	}

	if err := adminIdentity.RevokeCert(uuid); err != nil {
		printer.Fail("\r\n[*] Revoke failed: %s", err.Error())
		console.ready <- true
		return
	}
	if err := adminIdentity.Save(); err != nil {
		printer.Fail("\r\n[*] Failed to save identity: %s", err.Error())
		console.ready <- true
		return
	}

	handler.LetShutdown(route, uuid)
	printer.Success("\r\n[*] Certificate revoked and node shutdown sent")
	console.ready <- true
}
