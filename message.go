package pipe

import (
	"context"
	"errors"

	"github.com/Wondertan/go-libp2p-pipe/pb"
)

// for pipe.go not to import "pb" package
const (
	request  = pb.Message_REQ
	response = pb.Message_RESP
)

// NewRequest creates new Message with request form.
// Requests allow users to send any data through the Pipe
// and asynchronously receive responses at any time after other end calculates them.
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
// Pipe just them pass through without additional handling.
func NewMessage(msg []byte) *Message {
	return &Message{
		pb: pb.Message{
			Tag:  pb.Message_SIM,
			Body: msg,
		},
	}
}

// newResponse creates new Message of response form.
// Only internal use allowed.
// To send responses, use Reply method on request messages.
func newResponse(id uint64) *Message {
	return &Message{
		pb: pb.Message{
			Id:  id,
			Tag: pb.Message_RESP,
		},
	}
}

// Message is the central object in Pipe's communication.
// It is responsible for transferring Pipe's user data.
// Also, it has 3 forms: simple message, request, response.
type Message struct {
	pb pb.Message

	ctx  context.Context
	resp chan *Message
}

// Data returns bytes which were transported through message
func (r *Message) Data() []byte {
	return r.pb.Body
}

// Response waits and returns response, if the message is a sent request
func (r *Message) Response(ctx context.Context) ([]byte, error) {
	if r.resp == nil {
		return nil, errors.New("the message is not a request")
	}

	// TODO Handle reset ctx
	select {
	case m := <-r.resp:
		return m.response()
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (r *Message) response() ([]byte, error) {
	if r.pb.Err != "" {
		return r.pb.Body, errors.New(r.pb.Err)
	}

	return r.pb.Body, nil
}

// Data is an reply option which sends a byte slice to the requester as a response
func Data(data []byte) RepOpt {
	return func(m *Message) {
		if len(data) == 0 {
			return
		}

		m.pb.Body = data
	}
}

// Error is an reply option which sends an error to the requester as a response
func Error(err error) RepOpt {
	return func(m *Message) {
		if err == nil {
			return
		}

		m.pb.Err = err.Error()
	}
}

// Reply sends response, if the message is a received request
// Reply accepts different options as responses, which could be concatenated together
func (r *Message) Reply(resp ...RepOpt) error {
	if r.resp == nil {
		return errors.New("message is not a request")
	}

	if len(resp) == 0 {
		return errors.New("reply should not be empty")
	}

	// make response with the same id
	// for pipe being able to match it with request on other side
	msg := newResponse(r.pb.Id)

	for _, opt := range resp {
		opt(msg)
	}

	select {
	case r.resp <- msg:
		return nil
	case <-r.ctx.Done():
		return ErrClosed
	}
}

// RepOpt is a type of option functions for Reply method
type RepOpt func(*Message)
