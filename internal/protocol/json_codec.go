// Package protocol provides JSON codec implementation.
package protocol

import (
	"encoding/json"
)

// JSONCodec implements Codec interface using JSON encoding.
type JSONCodec struct{}

// NewJSONCodec creates a new JSONCodec instance.
func NewJSONCodec() *JSONCodec {
	return &JSONCodec{}
}

// Encode encodes a value into JSON bytes.
func (c *JSONCodec) Encode(v any) ([]byte, error) {
	return json.Marshal(v)
}

// Decode decodes JSON bytes into a value.
func (c *JSONCodec) Decode(data []byte, v any) error {
	return json.Unmarshal(data, v)
}
