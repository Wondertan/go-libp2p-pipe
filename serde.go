package pipe

import (
	"io"

	"github.com/Wondertan/go-serde"
)

// WriteMessage writes given Message to the Writer
func WriteMessage(w io.Writer, msg *Message) error {
	return serde.WriteMessage(w, &msg.pb)
}

// MarshalMessage fills given byte slice with the Message
func MarshalMessage(msg *Message, buf []byte) (int, error) {
	return serde.MarshalMessage(&msg.pb, buf)
}

// ReadMessage fills given Message from Reader
func ReadMessage(r io.Reader, msg *Message) error {
	return serde.ReadMessage(r, &msg.pb)
}

// UnmarshalMessage fills given Message from byte slice
func UnmarshalMessage(msg *Message, buf []byte) error {
	return serde.UnmarshalMessage(&msg.pb, buf)
}
