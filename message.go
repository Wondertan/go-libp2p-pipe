package pipe

import (
	"context"
	"errors"

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

func newResponse(id uint64) *Message {
	return &Message{
		pb: pb.Message{
			Id:  id,
			Tag: pb.Message_RESP,
		},
	}
}

type Message struct {
	pb pb.Message

	closeCtx context.Context
	resp     chan *Message
}

// Data returns bytes which were transported through message
func (r *Message) Data() []byte {
	return r.pb.Body
}

// Response waits for response, if the message is a sent request
func (r *Message) Response(ctx context.Context) ([]byte, error) {
	if r.resp == nil {
		return nil, errors.New("the message is not a request")
	}

	select {
	case m := <-r.resp:
		if m.pb.Err != "" {
			return m.pb.Body, errors.New(m.pb.Err)
		}
		return m.pb.Body, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func Data(data []byte) RepOpt {
	return func(m *Message) {
		if len(data) == 0 {
			return
		}
		m.pb.Body = data
	}
}

func Error(err error) RepOpt {
	return func(m *Message) {
		if err == nil {
			return
		}
		m.pb.Err = err.Error()
	}
}

// Reply sends response, if the message is a received request
func (r *Message) Reply(resp ...RepOpt) error {
	if r.resp == nil {
		return errors.New("message is not a request")
	}

	if len(resp) == 0 {
		return errors.New("reply should not be empty")
	}

	msg := newResponse(r.pb.Id)

	for _, opt := range resp {
		opt(msg)
	}

	r.resp <- msg

	return nil
}

type RepOpt func(*Message)
