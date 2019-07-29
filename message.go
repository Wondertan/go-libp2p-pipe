package pipe

import (
	"context"
	"encoding/binary"
	"errors"
	"io"

	pool "github.com/libp2p/go-buffer-pool"
	"github.com/libp2p/go-msgio"

	"github.com/Wondertan/go-libp2p-pipe/pb"
)

const (
	request  = pb.Message_REQ
	response = pb.Message_RESP
)

// NewRequest creates new pipe Message. After sending it waits for response from remote end of pipe
func NewRequest(msg []byte) *Message {
	return &Message{
		pb: pb.Message{
			Tag:  pb.Message_REQ,
			Body: msg,
		},
		resp: make(chan *Message, 1),
	}
}

// NewMessage creates new simple pipe Message.
func NewMessage(msg []byte) *Message {
	return &Message{
		pb: pb.Message{
			Tag:  pb.Message_MSG,
			Body: msg,
		},
	}
}

func newResponse(id uint64, msg []byte) *Message {
	return &Message{
		pb: pb.Message{
			Id:   id,
			Tag:  pb.Message_RESP,
			Body: msg,
		},
	}
}

type Message struct {
	pb pb.Message

	resp chan *Message
}

// Data returns bytes which were transported through message
func (r *Message) Data() []byte {
	return r.pb.Body
}

// GetResponse waits for response, if the message is a sent request
func (r *Message) GetResponse(ctx context.Context) ([]byte, error) {
	if r.resp == nil {
		return nil, errors.New("the message is not a request")
	}

	select {
	case m := <-r.resp:
		return m.pb.Body, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// SendResponse sends response, if the message is a received request
func (r *Message) SendResponse(msg []byte) {
	if r.resp == nil {
		return
	}

	r.resp <- newResponse(r.pb.Id, msg)
}

func readMessage(r io.Reader) (*Message, error) {
	mr := msgio.NewVarintReader(r)
	b, err := mr.ReadMsg()
	if err != nil && err != io.EOF {
		return nil, err
	}

	msg, err := unmarshalMessage(b)
	mr.ReleaseMsg(b)
	if err != nil {
		return nil, err
	}

	return msg, nil
}

func writeMessage(w io.Writer, msg *Message) error {
	b, err := marshalMessage(msg)
	if err != nil {
		return err
	}

	_, err = w.Write(b)
	return err
}

func unmarshalMessage(msg []byte) (*Message, error) {
	var pb pb.Message
	err := pb.Unmarshal(msg)
	if err != nil {
		return nil, err
	}

	return &Message{pb: pb}, nil
}

func marshalMessage(msg *Message) ([]byte, error) {
	size := msg.pb.Size()

	buf := pool.Get(size + binary.MaxVarintLen64)
	defer pool.Put(buf)

	n := binary.PutUvarint(buf, uint64(size))
	n2, err := msg.pb.MarshalTo(buf[n:])
	if err != nil {
		return nil, err
	}
	n += n2

	return buf[:n], nil
}
