package pipe

import (
	"context"
	"errors"
	"strings"

	logging "github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p-core"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
)

var log = logging.Logger("pipe")

type pipe struct {
	host host.Host

	peer   peer.ID
	s      network.Stream
	wtries int

	counter uint64
	msg     map[uint64]chan *Message

	ingoing  chan *Message
	outgoing chan *Message

	read  chan *Message
	resps chan *Message

	ctx    context.Context
	cancel context.CancelFunc
}

// NewPipe creates new pipe over new stream
func NewPipe(ctx context.Context, host core.Host, peer peer.ID, proto core.ProtocolID) (Pipe, error) {
	// stream is created inside to ensure that pipe takes full control over stream
	s, err := host.NewStream(ctx, peer, wrapProto(proto))
	if err != nil {
		return nil, err
	}

	return newPipe(ctx, s, host), nil
}

// SetPipeHandler sets new stream handler which wraps stream into the pipe
func SetPipeHandler(host core.Host, h Handler, proto core.ProtocolID) {
	host.SetStreamHandler(wrapProto(proto), func(stream network.Stream) {
		h(newPipe(context.TODO(), stream, host))
	})
}

// RemovePipeHandler removes pipe handler from host
func RemovePipeHandler(host core.Host, proto core.ProtocolID) {
	host.RemoveStreamHandler(wrapProto(proto))
}

func newPipe(ctx context.Context, s network.Stream, host host.Host) *pipe {
	ctx, cancel := context.WithCancel(ctx)

	p := &pipe{
		host:     host,
		peer:     s.Conn().RemotePeer(),
		s:        s,
		msg:      make(map[uint64]chan *Message),
		ingoing:  make(chan *Message, MessageBuffer),
		outgoing: make(chan *Message, 8),
		read:     make(chan *Message),
		resps:    make(chan *Message),
		ctx:      ctx,
		cancel:   cancel,
	}

	go p.handlingLoop()
	go p.handleRead()

	return p
}

func (p *pipe) Send(msg *Message) error {
	if p.isClosed() {
		return errors.New("can't send through closed pipe")
	}

	p.outgoing <- msg

	return nil
}

func (p *pipe) Next(ctx context.Context) (*Message, error) {
	if p.isClosed() {
		return nil, errors.New("can't read from closed pipe")
	}

	select {
	case m := <-p.ingoing:
		return m, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-p.ctx.Done():
		return nil, p.ctx.Err()
	}
}

func (p *pipe) Protocol() protocol.ID {
	return p.s.Protocol()
}

func (p *pipe) Conn() network.Conn {
	return p.s.Conn()
}

func (p *pipe) Close() error {
	p.cancel()
	// TODO Handle two side closing
	return p.s.Reset()
}

func (p *pipe) handlingLoop() {
	for {
		select {
		case msg := <-p.outgoing:
			p.handleOutgoing(msg)
		case msg := <-p.resps:
			p.handleWrite(msg)
		case msg := <-p.read:
			p.handleIngoing(msg)
		case <-p.ctx.Done():
			p.Close()
			return
		}
	}
}

func (p *pipe) handleOutgoing(msg *Message) {
	msg.pb.Id = p.counter
	p.counter++

	if msg.pb.Tag == request {
		p.msg[msg.pb.Id] = msg.resp
	}

	p.handleWrite(msg)
}

func (p *pipe) handleIngoing(msg *Message) {
	switch msg.pb.Tag {
	case response:
		resp, ok := p.msg[msg.pb.Id]
		if ok {
			resp <- msg
			delete(p.msg, msg.pb.Id)
			return
		}

		log.Info("Received response for unknown request, dropping...")
		return
	case request:
		msg.resp = p.resps
	}

	select {
	case p.ingoing <- msg:
	default:
		log.Info("Can't deliver message, messages are not being handled with `Next`")
	}
}

func (p *pipe) handleRead() {
	for {
		msg, err := readMessage(p.s)
		if err != nil {
			p.s.Reset()

			// TODO Handle restream
			log.Errorf("error reading from stream: %s", err)
			return
		}

		select {
		case p.read <- msg:
		case <-p.ctx.Done():
			return
		}
	}
}

func (p *pipe) handleWrite(msg *Message) {
	for {
		err := writeMessage(p.s, msg)
		if err == nil || p.wtries == MaxWriteAttempts {
			p.wtries = 0
			return
		}

		p.wtries++
		log.Errorf("error writing to stream: %s", err)
		return
	}
}

func (p *pipe) isClosed() bool {
	return p.ctx.Err() != nil
}

func wrapProto(proto protocol.ID) protocol.ID {
	if strings.HasPrefix(string(proto), "/") {
		return Protocol + proto
	}

	return Protocol + "/" + proto
}
