package pipe

import (
	"context"
	"errors"
	"io"
	"sync"

	logging "github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/protocol"
)

var log = logging.Logger("pipe")

var (
	ErrClosed = errors.New("pipe: closed")
	ErrReset  = errors.New("pipe: reset")
	ErrEmpty  = errors.New("pipe: empty message")
)

type pipe struct {
	proto protocol.ID

	in  network.Stream
	out network.Stream

	ingoing  chan *Message
	outgoing chan *Message

	counter uint64
	reqs    map[uint64]chan *Message
	l       sync.Mutex

	reset chan struct{}
	close chan struct{}
}

func newPipe(proto protocol.ID, in, out network.Stream) *pipe {
	p := &pipe{
		proto:    proto,
		in:       in,
		out:      out,
		ingoing:  make(chan *Message, MessageBufferSize),
		outgoing: make(chan *Message, MessageBufferSize),
		reqs:     make(map[uint64]chan *Message),
		reset:    make(chan struct{}),
		close:    make(chan struct{}),
	}

	go p.handleRead()
	go p.handleWrite()
	return p
}

func (p *pipe) isReset() bool {
	select {
	case <-p.reset:
		return true
	default:
		return false
	}
}

func (p *pipe) isClosed() bool {
	select {
	case <-p.close:
		return true
	default:
		return false
	}
}

func (p *pipe) Send(msg *Message) error {
	if p.isReset() {
		return ErrReset
	}
	if p.isClosed() {
		return ErrClosed
	}
	if isEmpty(msg) {
		return ErrEmpty
	}

	msg.p = p
	p.outgoing <- msg // TODO change to unbounded channel to make write non-blocking
	return nil
}

func (p *pipe) Next(ctx context.Context) (*Message, error) {
	// to ensure that ingoing is fully read, in case context is done or pipe reset.
	select {
	case m, ok := <-p.ingoing:
		if !ok {
			return nil, io.EOF
		}
		return m, nil
	default:
	}

	select {
	case m, ok := <-p.ingoing:
		if !ok {
			return nil, io.EOF
		}
		return m, nil
	case <-p.reset:
		return nil, ErrReset
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (p *pipe) Protocol() protocol.ID {
	return p.proto
}

func (p *pipe) Conn() network.Conn {
	return p.in.Conn()
}

func (p *pipe) Close() error {
	if p.isClosed() {
		return ErrReset
	}
	if p.isReset() {
		return nil
	}

	close(p.outgoing)
	close(p.close)
	return nil
}

func (p *pipe) Reset() error {
	if p.isReset() {
		return nil
	}

	close(p.reset)
	return nil
}

func (p *pipe) handleIngoing(msg *Message) {
	msg.p = p
	switch msg.pb.Tag {
	case response:
		p.l.Lock()
		defer p.l.Unlock()

		resp, ok := p.reqs[msg.pb.Id]
		if ok {
			resp <- msg // resp always bufferized, so this won't block
			delete(p.reqs, msg.pb.Id)
			return
		}

		log.Warn("Received response for unknown request, dropping...")
		return
	}

	select {
	case p.ingoing <- msg:
	default:
		log.Warn("Can't deliver message, messages are not being handled with `Next`, dropping...")
	}
}

func (p *pipe) handleWrite() {
	defer p.out.Reset()

	var err error
	for {
		select {
		case msg, ok := <-p.outgoing:
			if !ok {
				p.out.Close()
				return
			}

			if msg.pb.Tag == request {
				p.l.Lock()
				msg.pb.Id = p.counter
				p.reqs[msg.pb.Id] = msg.resp
				p.counter++
				p.l.Unlock()
			}

			err = WriteMessage(p.out, msg)
			if err != nil {
				log.Errorf("writing to stream: %s", err)
				p.Reset()
				return
			}
		case <-p.reset:
			return
		}
	}
}

func (p *pipe) handleRead() {
	defer p.in.Reset()

	var err error
	for {
		select {
		case <-p.reset:
			return
		default:
		}

		msg := new(Message)
		err = ReadMessage(p.in, msg)
		if err != nil {
			if err == io.EOF {
				p.in.Close()
				close(p.ingoing)
				return
			}

			log.Errorf("reading from stream: %s", err)
			p.Reset()
			return
		}

		p.handleIngoing(msg)
	}
}
