// Package protocol provides tests for custom protocol implementation.
package protocol

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCustomProtocol_EncodeDecode(t *testing.T) {
	codec := NewJSONCodec()
	protocol := NewCustomProtocol(codec)

	type TestMessage struct {
		Message string `json:"message"`
	}

	msg := TestMessage{Message: "hello"}

	// Test encode
	data, err := protocol.Encode(MessageTypeRequest, 123, &msg)
	require.NoError(t, err)
	require.NotEmpty(t, data)

	// Test decode
	var decoded TestMessage
	err = protocol.Decode(data, &decoded)
	require.NoError(t, err)
	assert.Equal(t, "hello", decoded.Message)
}

func TestCustomProtocol_InvalidMagicNumber(t *testing.T) {
	codec := NewJSONCodec()
	protocol := NewCustomProtocol(codec)

	// Create invalid data with wrong magic number
	invalidData := make([]byte, ProtocolHeaderSize)
	invalidData[0] = 0xFF // Wrong magic number

	var decoded any
	err := protocol.Decode(invalidData, &decoded)
	assert.Error(t, err)
	assert.Equal(t, ErrInvalidMagicNumber, err)
}

func TestParseHeader(t *testing.T) {
	// Create valid header data
	data := make([]byte, ProtocolHeaderSize)
	data[0] = 0x54 // T
	data[1] = 0x43 // C
	data[2] = 0x50 // P
	data[3] = 0x53 // S
	data[4] = 0x01 // Version

	header, err := ParseHeader(data)
	require.NoError(t, err)
	assert.Equal(t, uint32(MagicNumber), header.Magic)
	assert.Equal(t, uint8(0x01), header.Version)
}

func TestParseHeader_InvalidMagicNumber(t *testing.T) {
	// Create invalid header data
	data := make([]byte, ProtocolHeaderSize)
	data[0] = 0xFF // Wrong magic number

	_, err := ParseHeader(data)
	assert.Error(t, err)
	assert.Equal(t, ErrInvalidMagicNumber, err)
}

func TestParseHeader_InvalidSize(t *testing.T) {
	// Create data smaller than header size
	data := make([]byte, ProtocolHeaderSize-1)

	_, err := ParseHeader(data)
	assert.Error(t, err)
	assert.Equal(t, ErrInvalidMessageSize, err)
}
