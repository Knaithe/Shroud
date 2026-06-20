package crypto

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
)

const MaxDecompressedLen int64 = 64 << 20 // 64 MB

func GzipCompress(src []byte) []byte {
	var in bytes.Buffer
	w := gzip.NewWriter(&in)
	w.Write(src)
	w.Close()
	return in.Bytes()
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
