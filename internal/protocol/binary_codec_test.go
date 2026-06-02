// Package protocol provides tests for binary codec implementation.
package protocol

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBinaryCodec_Encode(t *testing.T) {
	codec := NewBinaryCodec()

	type TestMessage struct {
		Message string
		Count   int
	}

	msg := TestMessage{Message: "hello", Count: 42}

	data, err := codec.Encode(&msg)
	require.NoError(t, err)
	require.NotEmpty(t, data)
}

func TestBinaryCodec_Decode(t *testing.T) {
	codec := NewBinaryCodec()

	type TestMessage struct {
		Message string
		Count   int
	}

	// First encode a message
	original := TestMessage{Message: "hello", Count: 42}
	data, err := codec.Encode(&original)
	require.NoError(t, err)

	// Then decode it
	var decoded TestMessage
	err = codec.Decode(data, &decoded)
	require.NoError(t, err)
	assert.Equal(t, "hello", decoded.Message)
	assert.Equal(t, 42, decoded.Count)
}

func TestBinaryCodec_RoundTrip(t *testing.T) {
	codec := NewBinaryCodec()

	type TestMessage struct {
		Message string
		Count   int
		Active  bool
	}

	original := TestMessage{Message: "test", Count: 100, Active: true}

	data, err := codec.Encode(&original)
	require.NoError(t, err)

	var decoded TestMessage
	err = codec.Decode(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, original.Message, decoded.Message)
	assert.Equal(t, original.Count, decoded.Count)
	assert.Equal(t, original.Active, decoded.Active)
}
