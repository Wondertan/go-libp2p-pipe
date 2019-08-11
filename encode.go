package pipe

import (
	"encoding/binary"
	"io"

	pool "github.com/libp2p/go-buffer-pool"
)

// WriteMessage writes given Message to the Writer
func WriteMessage(w io.Writer, msg *Message) error {
	size := msg.pb.Size()
	buf := pool.Get(size + binary.MaxVarintLen64)
	defer pool.Put(buf)

	n, err := MarshalMessage(msg, buf)
	if err != nil {
		return err
	}

	_, err = w.Write(buf[:n])
	return err
}

// MarshalMessage fills given byte slice with the Message
func MarshalMessage(msg *Message, buf []byte) (int, error) {
	size := msg.pb.Size()

	n := binary.PutUvarint(buf, uint64(size))
	n2, err := msg.pb.MarshalTo(buf[n:])
	if err != nil {
		return 0, err
	}
	n += n2

	return n, nil
}
