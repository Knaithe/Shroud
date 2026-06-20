package crypto

import (
	"bytes"
	"compress/gzip"
	"testing"
)

func TestGzipDecompress_ExactlyAtLimit(t *testing.T) {
	// Data exactly at 64 MB should succeed.
	size := MaxDecompressedLen

	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	chunk := make([]byte, 1<<20) // 1 MB of zeros
	written := int64(0)
	for written < size {
		n := int64(len(chunk))
		if written+n > size {
			n = size - written
		}
		if _, err := w.Write(chunk[:n]); err != nil {
			t.Fatalf("failed to write gzip data: %v", err)
		}
		written += n
	}
	w.Close()

	out, err := GzipDecompress(buf.Bytes())
	if err != nil {
		t.Fatalf("decompression of exactly 64 MB should succeed, got error: %v", err)
	}
	if int64(len(out)) != MaxDecompressedLen {
		t.Fatalf("expected %d bytes, got %d", MaxDecompressedLen, len(out))
	}
}

func TestGzipDecompress_TruncatedStream(t *testing.T) {
	// Compress valid data, then truncate the gzip stream mid-body so that
	// io.ReadAll hits a read error (not a header parse error).
	original := bytes.Repeat([]byte("abcdefghij"), 10000)
	compressed := GzipCompress(original)

	// Keep the valid gzip header but chop the body roughly in half.
	truncated := compressed[:len(compressed)/2]
	_, err := GzipDecompress(truncated)
	if err == nil {
		t.Fatal("expected error when decompressing truncated gzip stream, got nil")
	}
}

func TestGzipDecompress_NotGzip(t *testing.T) {
	// Completely invalid data (not even a gzip header).
	_, err := GzipDecompress([]byte("this is not gzip"))
	if err == nil {
		t.Fatal("expected error for non-gzip input")
	}
}

func TestGzipRoundtrip(t *testing.T) {
	original := []byte("hello, this is a normal roundtrip test for gzip compression")
	compressed := GzipCompress(original)
	decompressed, err := GzipDecompress(compressed)
	if err != nil {
		t.Fatalf("unexpected decompression error: %v", err)
	}
	if !bytes.Equal(original, decompressed) {
		t.Fatalf("roundtrip mismatch: got %q, want %q", decompressed, original)
	}
}

func TestGzipDecompress_ExceedsLimit(t *testing.T) {
	// Create data slightly larger than the 64 MB limit.
	// To avoid allocating 64 MB of real data, we compress a stream of zeros
	// which compresses extremely well.
	size := MaxDecompressedLen + 1

	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	// Write in chunks to avoid a single huge allocation.
	chunk := make([]byte, 1<<20) // 1 MB of zeros
	written := int64(0)
	for written < size {
		n := int64(len(chunk))
		if written+n > size {
			n = size - written
		}
		if _, err := w.Write(chunk[:n]); err != nil {
			t.Fatalf("failed to write gzip data: %v", err)
		}
		written += n
	}
	w.Close()

	_, err := GzipDecompress(buf.Bytes())
	if err == nil {
		t.Fatal("expected error when decompressed data exceeds 64 MB limit, got nil")
	}
}

func TestGzipRoundtrip_Empty(t *testing.T) {
	original := []byte{}
	compressed := GzipCompress(original)
	decompressed, err := GzipDecompress(compressed)
	if err != nil {
		t.Fatalf("unexpected decompression error: %v", err)
	}
	if len(decompressed) != 0 {
		t.Fatalf("expected empty result, got %d bytes", len(decompressed))
	}
}

func TestGzipDecompress_CorruptData(t *testing.T) {
	corrupted := []byte{0x1f, 0x8b, 0x08, 0x00, 0xff, 0xff, 0xde, 0xad, 0xbe, 0xef}
	_, err := GzipDecompress(corrupted)
	if err == nil {
		t.Fatal("expected error when decompressing corrupted gzip data, got nil")
	}
}
