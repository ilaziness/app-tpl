// Package protocol provides custom binary protocol implementation for TCP/UDP.
package protocol

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
)

const (
	// MagicNumber is the protocol magic number for identification.
	MagicNumber = 0x54435053 // "TCPS" in hex

	// ProtocolHeaderSize is the fixed size of the protocol header.
	ProtocolHeaderSize = 20 // 4(magic) + 1(version) + 1(type) + 4(msgID) + 1(codec) + 1(reserved) + 4(length) + 4(checksum)

	// ProtocolVersion is the current protocol version.
	ProtocolVersion = 0x01

	// Message types
	MessageTypeRequest   = 0x01
	MessageTypeResponse  = 0x02
	MessageTypeHeartbeat = 0x03
	MessageTypeError     = 0x04

	// Codec types
	CodecTypeJSON   = 0x00
	CodecTypeBinary = 0x01
)

var (
	ErrInvalidMagicNumber = errors.New("invalid magic number")
	ErrInvalidChecksum    = errors.New("invalid checksum")
	ErrInvalidMessageSize = errors.New("invalid message size")
)

// ProtocolHeader represents the custom protocol header.
type ProtocolHeader struct {
	Magic      uint32 // 4 bytes: Magic number (0x54435053)
	Version    uint8  // 1 byte: Protocol version
	Type       uint8  // 1 byte: Message type
	MessageID  uint32 // 4 bytes: Message ID for request-response matching
	Codec      uint8  // 1 byte: Serialization type (0=JSON, 1=Binary)
	Reserved   uint8  // 1 byte: Reserved for future use
	DataLength uint32 // 4 bytes: Payload length
	Checksum   uint32 // 4 bytes: CRC32 checksum
}

// Message represents a complete protocol message.
type Message struct {
	Header ProtocolHeader
	Data   []byte
}

// CustomProtocol implements the custom binary protocol encoding/decoding.
type CustomProtocol struct {
	codec     Codec
	codecType uint8
}

// NewCustomProtocol creates a new CustomProtocol instance.
func NewCustomProtocol(codec Codec) *CustomProtocol {
	var codecType uint8 = CodecTypeJSON
	if _, ok := codec.(*BinaryCodec); ok {
		codecType = CodecTypeBinary
	}
	return &CustomProtocol{codec: codec, codecType: codecType}
}

// Encode encodes a message into bytes with protocol header.
func (p *CustomProtocol) Encode(msgType uint8, messageID uint32, v any) ([]byte, error) {
	// Encode payload using the configured codec
	payload, err := p.codec.Encode(v)
	if err != nil {
		return nil, err
	}

	// Build protocol header
	header := ProtocolHeader{
		Magic:      MagicNumber,
		Version:    ProtocolVersion,
		Type:       msgType,
		MessageID:  messageID,
		Codec:      p.codecType,
		Reserved:   0x00,
		DataLength: uint32(len(payload)), //nolint:gosec // payload length bounded by protocol max size
	}

	// Calculate checksum
	checksum := crc32.ChecksumIEEE(payload)
	header.Checksum = checksum

	// Serialize header
	buf := new(bytes.Buffer)
	buf.Grow(ProtocolHeaderSize + len(payload))

	if err := binary.Write(buf, binary.BigEndian, header.Magic); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.BigEndian, header.Version); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.BigEndian, header.Type); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.BigEndian, header.MessageID); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.BigEndian, header.Codec); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.BigEndian, header.Reserved); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.BigEndian, header.DataLength); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.BigEndian, header.Checksum); err != nil {
		return nil, err
	}

	// Append payload
	if _, err := buf.Write(payload); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// Decode decodes bytes into a message with protocol header validation.
func (p *CustomProtocol) Decode(data []byte, v any) error {
	if len(data) < ProtocolHeaderSize {
		return ErrInvalidMessageSize
	}

	// Parse header
	buf := bytes.NewReader(data)
	header := ProtocolHeader{}

	if err := binary.Read(buf, binary.BigEndian, &header.Magic); err != nil {
		return err
	}
	if err := binary.Read(buf, binary.BigEndian, &header.Version); err != nil {
		return err
	}
	if err := binary.Read(buf, binary.BigEndian, &header.Type); err != nil {
		return err
	}
	if err := binary.Read(buf, binary.BigEndian, &header.MessageID); err != nil {
		return err
	}
	if err := binary.Read(buf, binary.BigEndian, &header.Codec); err != nil {
		return err
	}
	if err := binary.Read(buf, binary.BigEndian, &header.Reserved); err != nil {
		return err
	}
	if err := binary.Read(buf, binary.BigEndian, &header.DataLength); err != nil {
		return err
	}
	if err := binary.Read(buf, binary.BigEndian, &header.Checksum); err != nil {
		return err
	}

	// Validate magic number
	if header.Magic != MagicNumber {
		return ErrInvalidMagicNumber
	}

	// Validate protocol version
	if header.Version != ProtocolVersion {
		return fmt.Errorf("unsupported protocol version: %d, expected %d", header.Version, ProtocolVersion)
	}

	// Validate codec type matches configured codec
	if header.Codec != p.codecType {
		return fmt.Errorf("codec type mismatch: expected %d, got %d", p.codecType, header.Codec)
	}

	// Validate data length
	if uint32(len(data)-ProtocolHeaderSize) != header.DataLength { //nolint:gosec // validated against header
		return ErrInvalidMessageSize
	}

	// Extract payload
	payload := data[ProtocolHeaderSize:]

	// Validate checksum
	calculatedChecksum := crc32.ChecksumIEEE(payload)
	if calculatedChecksum != header.Checksum {
		return ErrInvalidChecksum
	}

	// Decode payload using the configured codec
	return p.codec.Decode(payload, v)
}

// ParseHeader parses only the protocol header from bytes.
func ParseHeader(data []byte) (*ProtocolHeader, error) {
	if len(data) < ProtocolHeaderSize {
		return nil, ErrInvalidMessageSize
	}

	buf := bytes.NewReader(data)
	header := &ProtocolHeader{}

	if err := binary.Read(buf, binary.BigEndian, &header.Magic); err != nil {
		return nil, err
	}
	if err := binary.Read(buf, binary.BigEndian, &header.Version); err != nil {
		return nil, err
	}
	if err := binary.Read(buf, binary.BigEndian, &header.Type); err != nil {
		return nil, err
	}
	if err := binary.Read(buf, binary.BigEndian, &header.MessageID); err != nil {
		return nil, err
	}
	if err := binary.Read(buf, binary.BigEndian, &header.Codec); err != nil {
		return nil, err
	}
	if err := binary.Read(buf, binary.BigEndian, &header.Reserved); err != nil {
		return nil, err
	}
	if err := binary.Read(buf, binary.BigEndian, &header.DataLength); err != nil {
		return nil, err
	}
	if err := binary.Read(buf, binary.BigEndian, &header.Checksum); err != nil {
		return nil, err
	}

	if header.Magic != MagicNumber {
		return nil, ErrInvalidMagicNumber
	}

	return header, nil
}
