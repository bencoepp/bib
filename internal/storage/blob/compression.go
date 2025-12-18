package blob

import (
	"compress/gzip"
	"fmt"
	"io"

	"github.com/klauspost/compress/zstd"
)

// Compression utilities for blob storage.

// newCompressionWriter creates a compression writer based on the algorithm.
func newCompressionWriter(w io.Writer, algorithm string, level int) (io.WriteCloser, error) {
	switch algorithm {
	case "gzip":
		if level < gzip.DefaultCompression || level > gzip.BestCompression {
			level = gzip.DefaultCompression
		}
		return gzip.NewWriterLevel(w, level)

	case "zstd":
		encoderLevel := zstd.SpeedDefault
		switch {
		case level <= 3:
			encoderLevel = zstd.SpeedFastest
		case level <= 7:
			encoderLevel = zstd.SpeedDefault
		case level <= 15:
			encoderLevel = zstd.SpeedBetterCompression
		default:
			encoderLevel = zstd.SpeedBestCompression
		}
		return zstd.NewWriter(w, zstd.WithEncoderLevel(encoderLevel))

	case "none":
		return &nopWriteCloser{w}, nil

	default:
		return nil, fmt.Errorf("unsupported compression algorithm: %s", algorithm)
	}
}

// newDecompressionReader creates a decompression reader based on the algorithm.
func newDecompressionReader(r io.Reader, algorithm string) (io.ReadCloser, error) {
	switch algorithm {
	case "gzip":
		return gzip.NewReader(r)

	case "zstd":
		decoder, err := zstd.NewReader(r)
		if err != nil {
			return nil, err
		}
		return io.NopCloser(decoder.IOReadCloser()), nil

	case "none":
		return io.NopCloser(r), nil

	default:
		return nil, fmt.Errorf("unsupported compression algorithm: %s", algorithm)
	}
}

// nopWriteCloser wraps a Writer to add a no-op Close method.
type nopWriteCloser struct {
	io.Writer
}

func (nwc *nopWriteCloser) Close() error {
	return nil
}
