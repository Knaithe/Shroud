package process

import (
	"os"

	"Shroud/crypto"
	"Shroud/global"
	"Shroud/identity"
	"Shroud/utils"
)

var SelfDeleteOnExit bool

func CleanShutdown() { cleanShutdown() }

func cleanShutdown() {
	if global.Session != nil {
		if global.Session.AgentIdentity != nil {
			global.Session.AgentIdentity.WipeSeeds()
		}
		lk := global.Session.GetLinkKey()
		crypto.Wipe(lk)
	}
	if global.G_Component != nil {
		crypto.Wipe(global.G_Component.CryptoKey)
		if global.G_Component.Conn != nil {
			global.G_Component.Conn.Close()
		}
	}
	if SelfDeleteOnExit {
		if p := identity.DefaultAgentPath(); p != "" {
			utils.SecureRemoveFile(p)
		}
		utils.SelfDeleteBinary()
	}
	os.Exit(0)
}
