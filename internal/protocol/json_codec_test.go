// Package protocol provides tests for JSON codec implementation.
package protocol

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJSONCodec_Encode(t *testing.T) {
	codec := NewJSONCodec()

	type TestMessage struct {
		Message string `json:"message"`
		Count   int    `json:"count"`
	}

	msg := TestMessage{Message: "hello", Count: 42}

	data, err := codec.Encode(&msg)
	require.NoError(t, err)
	require.NotEmpty(t, data)
	assert.Contains(t, string(data), "hello")
	assert.Contains(t, string(data), "42")
}

func TestJSONCodec_Decode(t *testing.T) {
	codec := NewJSONCodec()

	type TestMessage struct {
		Message string `json:"message"`
		Count   int    `json:"count"`
	}

	data := []byte(`{"message":"hello","count":42}`)

	var decoded TestMessage
	err := codec.Decode(data, &decoded)
	require.NoError(t, err)
	assert.Equal(t, "hello", decoded.Message)
	assert.Equal(t, 42, decoded.Count)
}

func TestJSONCodec_Decode_InvalidJSON(t *testing.T) {
	codec := NewJSONCodec()

	var decoded any
	err := codec.Decode([]byte("invalid json"), &decoded)
	assert.Error(t, err)
}
