//go:build linux

package utils

import (
	"bytes"
	"os"
	"strconv"
	"strings"
	"unsafe"

	"golang.org/x/sys/unix"
)

func DisableCoreDump() {
	_ = unix.Prctl(unix.PR_SET_DUMPABLE, 0, 0, 0, 0)
}

func MaskProcessName(name string) {
	nameBytes := make([]byte, 16)
	copy(nameBytes, name)
	_ = unix.Prctl(unix.PR_SET_NAME, uintptr(unsafe.Pointer(&nameBytes[0])), 0, 0, 0)
	os.Args = []string{name}
}

func ScrubCmdline() {
	argStart, argEnd := getArgRange()
	if argStart == 0 || argEnd <= argStart || argEnd-argStart > 1<<20 {
		return
	}

	cmdline, err := os.ReadFile("/proc/self/cmdline")
	if err != nil || len(cmdline) == 0 {
		return
	}

	args := bytes.Split(cmdline, []byte{0})
	modified := false
	for i := 0; i < len(args); i++ {
		s := string(args[i])
		if (s == "-s" || s == "--passphrase") && i+1 < len(args) {
			for j := range args[i+1] {
				args[i+1][j] = 0
			}
			modified = true
			i++
		} else if strings.HasPrefix(s, "-s=") {
			for j := 3; j < len(args[i]); j++ {
				args[i][j] = 0
			}
			modified = true
		} else if strings.HasPrefix(s, "--passphrase=") {
			for j := 13; j < len(args[i]); j++ {
				args[i][j] = 0
			}
			modified = true
		}
	}
	if !modified {
		return
	}

	scrubbed := bytes.Join(args, []byte{0})
	mem, err := os.OpenFile("/proc/self/mem", os.O_WRONLY, 0)
	if err != nil {
		return
	}
	defer mem.Close()
	mem.WriteAt(scrubbed, int64(argStart))
}

func getArgRange() (uint64, uint64) {
	stat, err := os.ReadFile("/proc/self/stat")
	if err != nil {
		return 0, 0
	}
	s := string(stat)
	cp := strings.LastIndex(s, ")")
	if cp < 0 || cp+2 >= len(s) {
		return 0, 0
	}
	fields := strings.Fields(s[cp+2:])
	if len(fields) < 47 {
		return 0, 0
	}
	start, err1 := strconv.ParseUint(fields[45], 10, 64)
	end, err2 := strconv.ParseUint(fields[46], 10, 64)
	if err1 != nil || err2 != nil {
		return 0, 0
	}
	return start, end
}
