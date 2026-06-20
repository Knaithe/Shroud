package cli

import (
	"Shroud/admin/handler"
	"Shroud/admin/printer"
	"Shroud/share"
	"Shroud/utils"
)

func (console *Console) cmdUpload(fCommand []string, uuidNum int, uuid string, route string) {
	var err error

	file := console.mgr.FileManager.NewTransfer()
	file.FilePath, file.FileName, err = utils.ParseFileCommand(fCommand[1:])
	if err != nil {
		printer.Fail("\r\n[*] Error: %s", err.Error())
		console.mgr.FileManager.RemoveTransfer(file.TransferID)
		console.ready <- true
		return
	}

	err = file.SendFileStat(route, uuid, share.ADMIN)

	if err == nil && <-console.mgr.ConsoleManager.OK {
		go handler.StartBar(file.StatusChan, file.FileSize)
		file.Upload(route, uuid, share.ADMIN)
		console.mgr.FileManager.RemoveTransfer(file.TransferID)
	} else if err != nil {
		printer.Fail("\r\n[*] Unable to upload file, Error: %s", err.Error())
		console.mgr.FileManager.RemoveTransfer(file.TransferID)
	} else {
		printer.Fail("\r\n[*] Unable to create file, check %s status!", file.FileName)
		console.mgr.FileManager.RemoveTransfer(file.TransferID)
	}
	console.ready <- true
}

func (console *Console) cmdDownload(fCommand []string, uuidNum int, uuid string, route string) {
	var err error

	file := console.mgr.FileManager.NewTransfer()
	file.FilePath, file.FileName, err = utils.ParseFileCommand(fCommand[1:])
	if err != nil {
		printer.Fail("\r\n[*] Error: %s", err.Error())
		console.mgr.FileManager.RemoveTransfer(file.TransferID)
		console.ready <- true
		return
	}

	file.Ask4Download(route, uuid)

	if <-console.mgr.ConsoleManager.OK {
		err := file.CheckFileStat(route, uuid, share.ADMIN)
		if err == nil {
			go handler.StartBar(file.StatusChan, file.FileSize)
			file.Receive(route, uuid, share.ADMIN)
			console.mgr.FileManager.RemoveTransfer(file.TransferID)
		} else {
			printer.Fail("\r\n[*] Unable to create file, Error: %s", err.Error())
			console.mgr.FileManager.RemoveTransfer(file.TransferID)
		}
	} else {
		printer.Fail("\r\n[*] Unable to download file, check %s status!", file.FilePath)
		console.mgr.FileManager.RemoveTransfer(file.TransferID)
	}
	console.ready <- true
}
