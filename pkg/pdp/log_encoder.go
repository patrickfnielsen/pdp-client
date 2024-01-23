package pdp

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
)

// chunkEncoder implements log buffer chunking and compression. Log events are
// written to the encoder and the encoder outputs chunks that are fit to the
// configured limit.
type chunkEncoder struct {
	flushLimit   int64
	bytesWritten int
	buf          *bytes.Buffer
	w            *gzip.Writer
}

func newChunkEncoder(limit int64) *chunkEncoder {
	enc := &chunkEncoder{
		flushLimit: limit,
	}
	enc.update()

	return enc
}

func (enc *chunkEncoder) Write(event DecisionResult) (result [][]byte, err error) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(event); err != nil {
		return nil, err
	}

	bs := buf.Bytes()

	if len(bs) == 0 {
		return nil, nil
	}

	if int64(len(bs)+enc.bytesWritten+1) > enc.flushLimit {
		if err := enc.writeClose(); err != nil {
			return nil, err
		}

		result = enc.update()
	}

	if enc.bytesWritten == 0 {
		n, err := enc.w.Write([]byte(`[`))
		if err != nil {
			return nil, err
		}
		enc.bytesWritten += n
	} else {
		n, err := enc.w.Write([]byte(`,`))
		if err != nil {
			return nil, err
		}
		enc.bytesWritten += n
	}

	n, err := enc.w.Write(bs)
	if err != nil {
		return nil, err
	}

	enc.bytesWritten += n
	return
}

func (enc *chunkEncoder) writeClose() error {
	if _, err := enc.w.Write([]byte(`]`)); err != nil {
		return err
	}
	return enc.w.Close()
}

func (enc *chunkEncoder) Flush() ([][]byte, error) {
	if enc.bytesWritten == 0 {
		return nil, nil
	}
	if err := enc.writeClose(); err != nil {
		return nil, err
	}
	return enc.update(), nil
}

func (enc *chunkEncoder) update() [][]byte {
	buf := enc.buf
	enc.initialize()
	if buf != nil {
		return [][]byte{buf.Bytes()}
	}
	return nil
}

func (enc *chunkEncoder) initialize() {
	enc.buf = new(bytes.Buffer)
	enc.bytesWritten = 0
	enc.w = gzip.NewWriter(enc.buf)
}
