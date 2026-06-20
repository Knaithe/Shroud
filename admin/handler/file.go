package handler

import (
	"Shroud/admin/manager"
	"Shroud/protocol"
	"Shroud/share"

	"github.com/cheggaaa/pb"
)

// generate new bar
func NewBar(length int64) *pb.ProgressBar {
	bar := pb.New64(int64(length))
	bar.SetTemplate(pb.Full)
	bar.Set(pb.Bytes, true)

	return bar
}

func StartBar(statusChan chan *share.Status, size int64) {
	bar := NewBar(size)

	for {
		status := <-statusChan
		switch status.Stat {
		case share.START:
			bar.Start()
		case share.ADD:
			bar.Add64(status.Scale)
		case share.DONE:
			bar.Finish()
			return
		}
	}
}

func DispatchFileMess(mgr *manager.Manager) {
	for {
		message := <-mgr.FileManager.FileMessChan

		switch mess := message.(type) {
		case *protocol.FileStatReq:
			file, ok := mgr.FileManager.GetTransfer(mess.TransferID)
			if !ok {
				continue
			}
			file.FileSize = int64(mess.FileSize)
			file.SliceNum = mess.SliceNum
			mgr.ConsoleManager.OK <- true
		case *protocol.FileStatRes:
			file, ok := mgr.FileManager.GetTransfer(mess.TransferID)
			if !ok {
				continue
			}
			if mess.OK == 1 {
				mgr.ConsoleManager.OK <- true
			} else {
				file.Handler.Close()
				mgr.FileManager.RemoveTransfer(mess.TransferID)
				mgr.ConsoleManager.OK <- false
			}
		case *protocol.FileDownRes:
			mgr.ConsoleManager.OK <- false
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
