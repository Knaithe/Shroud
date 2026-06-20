package utils

import (
	"crypto/rand"
	"os"
)

func SecureRemoveFile(path string) error {
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return os.Remove(path)
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return os.Remove(path)
	}
	junk := make([]byte, 4096)
	remaining := info.Size()
	for remaining > 0 {
		n := int64(len(junk))
		if n > remaining {
			n = remaining
		}
		rand.Read(junk[:n])
		f.Write(junk[:n])
		remaining -= n
	}
	f.Sync()
	f.Close()
	return os.Remove(path)
}
