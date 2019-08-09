package pipe

import (
	"io"

	"github.com/libp2p/go-msgio"
)

func readMessage(r io.Reader, msg *Message) error {
	mr := msgio.NewVarintReader(r)
	b, err := mr.ReadMsg()
	if err != nil {
		return err
	}

	err = unmarshalMessage(msg, b)
	mr.ReleaseMsg(b)
	if err != nil {
		return err
	}

	return nil
}

func unmarshalMessage(msg *Message, buf []byte) error {
	err := msg.pb.Unmarshal(buf)
	if err != nil {
		return err
	}

	return nil
}
