package pipe

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"

	logging "github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p-core"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
)

var log = logging.Logger("pipe")

var (
	ErrClosed = errors.New("pipe: closed")
	ErrReset  = errors.New("pipe: reset")
	ErrEmpty  = errors.New("pipe: empty message")
)

type pipe struct {
	host host.Host

	s     network.Stream
	tries int

	counter uint64
	msg     map[uint64]chan *Message

	ingoing chan *Message
	read    chan *Message

	resetCtx context.Context
	reset    context.CancelFunc
	closeCtx context.Context
	close    context.CancelFunc

	l sync.Mutex
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
	resetCtx, reset := context.WithCancel(ctx)
	closeCtx, close := context.WithCancel(ctx)

	p := &pipe{
		host:     host,
		s:        s,
		msg:      make(map[uint64]chan *Message),
		ingoing:  make(chan *Message, MessageBuffer),
		read:     make(chan *Message, 8),
		resetCtx: resetCtx,
		reset:    reset,
		closeCtx: closeCtx,
		close:    close,
	}

	go p.handleRead()

	return p
}

func (p *pipe) Send(ctx context.Context, msg *Message) (err error) {
	if p.isClosed() {
		return ErrClosed
	}
	if isEmpty(msg) {
		return ErrEmpty
	}

	ctx = log.Start(ctx, "Send")
	defer func() {
		if err != nil {
			log.FinishWithErr(ctx, err)
		}
		log.Finish(ctx)
	}()

	return p.handleOutgoing(ctx, msg)
}

func (p *pipe) Next(ctx context.Context) (*Message, error) {
	// to ensure that ingoing is fully read
	select {
	case m := <-p.ingoing:
		return m, nil
	default:
	}

	select {
	case m := <-p.ingoing:
		return m, nil
	case <-p.resetCtx.Done():
		return nil, ErrReset
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (p *pipe) Protocol() protocol.ID {
	return p.s.Protocol()
}

func (p *pipe) Conn() network.Conn {
	return p.s.Conn()
}

func (p *pipe) Close() error {
	if p.isClosed() {
		return ErrClosed
	}

	p.l.Lock()
	defer p.l.Unlock()

	p.close()
	return nil // p.s.Close()
}

func (p *pipe) Reset() error {
	if p.isReset() {
		return ErrReset
	}

	defer p.clear()

	p.l.Lock()
	defer p.l.Unlock()

	p.reset()
	return p.s.Reset()
}

func (p *pipe) clear() {
	p.msg = make(map[uint64]chan *Message)
}

func (p *pipe) handleOutgoing(ctx context.Context, msg *Message) error {
	p.l.Lock()
	defer p.l.Unlock()

	if msg.pb.Tag == request {
		msg.pb.Id = p.counter
		p.msg[msg.pb.Id] = msg.resp
		p.counter++
	}

	return p.handleWrite(ctx, msg)
}

func (p *pipe) handleIngoing(msg *Message) {
	p.l.Lock()
	defer p.l.Unlock()

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
		msg.p = p
	}

	select {
	case p.ingoing <- msg:
	default:
		log.Info("Can't deliver message, messages are not being handled with `Next`")
	}
}

func (p *pipe) handleRead() {
	var err error
	for {
		msg := new(Message)
		err = ReadMessage(p.s, msg)
		if err != nil {
			if p.isClosed() {
				// fully close pipe if our end is already closed
				p.reset()
			}

			// not to log obvious errors
			if err != io.EOF {
				log.Errorf("error reading from pipe's stream: %s", err)
			}

			return
		}

		p.handleIngoing(msg)
	}
}

func (p *pipe) handleWrite(ctx context.Context, msg *Message) error {
	var err error
	for p.tries = 0; p.tries < MaxWriteAttempts; p.tries++ {
		err = WriteMessage(p.s, msg)
		if err == nil {
			log.LogKV(ctx, "msgID", msg.pb.Id)
			return nil
		}
	}

	return err
}

func (p *pipe) isClosed() bool {
	return p.closeCtx.Err() != nil || p.isReset()
}

func (p *pipe) isReset() bool {
	return p.resetCtx.Err() != nil
}

func wrapProto(proto protocol.ID) protocol.ID {
	if strings.HasPrefix(string(proto), "/") {
		return Protocol + proto
	}

	return Protocol + "/" + proto
}
