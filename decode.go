package pipe

import (
	"io"

	"github.com/libp2p/go-msgio"
)

// ReadMessage fills given Message from Reader
func ReadMessage(r io.Reader, msg *Message) error {
	mr := msgio.NewVarintReader(r)
	b, err := mr.ReadMsg()
	if err != nil {
		return err
	}

	err = UnmarshalMessage(msg, b)
	mr.ReleaseMsg(b)
	if err != nil {
		return err
	}

	return nil
}

// UnmarshalMessage fills given Message from byte slice
func UnmarshalMessage(msg *Message, b []byte) error {
	err := msg.pb.Unmarshal(b)
	if err != nil {
		return err
	}

	return nil
}
