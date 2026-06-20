package process

import (
	"os"

	"Shroud/crypto"
	"Shroud/global"
	"Shroud/utils"
)

var SelfDeleteOnExit bool

func cleanShutdown() {
	if global.Session != nil {
		if global.Session.AgentIdentity != nil {
			global.Session.AgentIdentity.WipeSeeds()
		}
		crypto.Wipe(global.Session.LinkKey)
	}
	if global.G_Component != nil {
		crypto.Wipe(global.G_Component.CryptoKey)
	}
	if SelfDeleteOnExit {
		utils.SelfDeleteBinary()
	}
	os.Exit(0)
}
