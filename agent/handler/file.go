package handler

import (
	"Shroud/agent/manager"
	"Shroud/protocol"
	"Shroud/share"
)

func DispatchFileMess(mgr *manager.Manager) {
	for {
		message := <-mgr.FileManager.FileMessChan

		switch mess := message.(type) {
		case *protocol.FileStatReq:
			// Incoming upload from admin: create a new transfer on the agent side
			// but use the admin's TransferID so responses route back correctly
			file := mgr.FileManager.NewTransfer()
			oldID := file.TransferID
			file.TransferID = mess.TransferID
			// Re-key: remove from auto-generated ID, store under admin's ID
			mgr.FileManager.RemoveTransfer(oldID)
			mgr.FileManager.StoreTransfer(mess.TransferID, file)
			file.FileName = mess.Filename
			file.SliceNum = mess.SliceNum
			err := file.CheckFileStat(protocol.TEMP_ROUTE, protocol.ADMIN_UUID, share.AGENT)
			if err == nil {
				go file.Receive(protocol.TEMP_ROUTE, protocol.ADMIN_UUID, share.AGENT)
			} else {
				mgr.FileManager.RemoveTransfer(file.TransferID)
			}
		case *protocol.FileStatRes:
			file, ok := mgr.FileManager.GetTransfer(mess.TransferID)
			if !ok {
				continue
			}
			if mess.OK == 1 {
				go file.Upload(protocol.TEMP_ROUTE, protocol.ADMIN_UUID, share.AGENT)
			} else {
				file.Handler.Close()
				mgr.FileManager.RemoveTransfer(mess.TransferID)
			}
		case *protocol.FileDownReq:
			// Incoming download request from admin: create a new transfer on the agent side
			// but use the admin's TransferID so responses route back correctly
			file := mgr.FileManager.NewTransfer()
			oldID := file.TransferID
			file.TransferID = mess.TransferID
			// Re-key: remove from auto-generated ID, store under admin's ID
			mgr.FileManager.RemoveTransfer(oldID)
			mgr.FileManager.StoreTransfer(mess.TransferID, file)
			file.FilePath = mess.FilePath
			file.FileName = mess.Filename
			go file.SendFileStat(protocol.TEMP_ROUTE, protocol.ADMIN_UUID, share.AGENT)
		case *protocol.FileData:
			file, ok := mgr.FileManager.GetTransfer(mess.TransferID)
			if !ok {
				continue
			}
			file.DataChan <- mess.Data
		case *protocol.FileErr:
			file, ok := mgr.FileManager.GetTransfer(mess.TransferID)
			if !ok {
				continue
			}
			file.ErrChan <- true
		}
	}
}
