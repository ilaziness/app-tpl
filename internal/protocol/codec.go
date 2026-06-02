// Package protocol provides codec interfaces for TCP/UDP protocol encoding and decoding.
package protocol

import "io"

// Encoder defines the interface for encoding data into bytes.
type Encoder interface {
	Encode(v any) ([]byte, error)
}

// Decoder defines the interface for decoding bytes into data.
type Decoder interface {
	Decode(data []byte, v any) error
}

// Codec combines Encoder and Decoder interfaces.
type Codec interface {
	Encoder
	Decoder
}

// StreamEncoder defines the interface for streaming encoding.
type StreamEncoder interface {
	EncodeTo(w io.Writer, v any) error
}

// StreamDecoder defines the interface for streaming decoding.
type StreamDecoder interface {
	DecodeFrom(r io.Reader, v any) error
}

// StreamCodec combines StreamEncoder and StreamDecoder interfaces.
type StreamCodec interface {
	StreamEncoder
	StreamDecoder
}
