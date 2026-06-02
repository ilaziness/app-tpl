// Package protocol provides binary codec implementation using Gob.
package protocol

import (
	"bytes"
	"encoding/gob"
)

// BinaryCodec implements Codec interface using Gob encoding.
type BinaryCodec struct{}

// NewBinaryCodec creates a new BinaryCodec instance.
func NewBinaryCodec() *BinaryCodec {
	return &BinaryCodec{}
}

// Encode encodes a value into Gob bytes.
func (c *BinaryCodec) Encode(v any) ([]byte, error) {
	var buf bytes.Buffer
	encoder := gob.NewEncoder(&buf)
	if err := encoder.Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Decode decodes Gob bytes into a value.
func (c *BinaryCodec) Decode(data []byte, v any) error {
	buf := bytes.NewReader(data)
	decoder := gob.NewDecoder(buf)
	return decoder.Decode(v)
}
