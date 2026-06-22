package crypto

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
)

const MaxDecompressedLen int64 = 64 << 20 // 64 MB

func GzipCompress(src []byte) []byte {
	out, _ := GzipCompressE(src)
	return out
}

func GzipCompressE(src []byte) ([]byte, error) {
	var in bytes.Buffer
	w := gzip.NewWriter(&in)
	if _, err := w.Write(src); err != nil {
		return nil, fmt.Errorf("gzip write: %w", err)
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("gzip close: %w", err)
	}
	return in.Bytes(), nil
}

func GzipDecompress(src []byte) ([]byte, error) {
	br := bytes.NewReader(src)
	gr, err := gzip.NewReader(br)
	if err != nil {
		return nil, fmt.Errorf("gzip reader: %w", err)
	}
	defer gr.Close()
	lr := &io.LimitedReader{R: gr, N: MaxDecompressedLen + 1}
	tmp, err := io.ReadAll(lr)
	if err != nil {
		return nil, fmt.Errorf("gzip read: %w", err)
	}
	if int64(len(tmp)) > MaxDecompressedLen {
		return nil, fmt.Errorf("decompressed data exceeds %d bytes limit", MaxDecompressedLen)
	}
	return tmp, nil
}
