package pipe

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
)

// TODO Figure out proper value
var HandshakeTimeout = time.Minute

var ErrHandshake = errors.New("pipe: handshake timeout")

type Pipeliner struct {
	host host.Host

	// TODO gc stale streams
	awaiting struct {
		sync.RWMutex
		m map[protocol.ID]map[peer.ID]chan network.Stream
	}
}

func NewPipeLiner(host host.Host) *Pipeliner {
	pl := &Pipeliner{
		host: host,
		awaiting: struct {
			sync.RWMutex
			m map[protocol.ID]map[peer.ID]chan network.Stream
		}{m: make(map[protocol.ID]map[peer.ID]chan network.Stream)},
	}

	host.SetStreamHandler(Protocol, pl.handleStream)
	return pl
}

func (pl *Pipeliner) NewPipe(ctx context.Context, proto protocol.ID, peer peer.ID) (p Pipe, err error) {
	pl.awaiting.RLock()
	if peers, ok := pl.awaiting.m[proto]; ok {
		if _, ok = peers[peer]; ok {
			pl.awaiting.RUnlock()
			// TODO Decide on keeping that limitation or not
			return nil, errors.New("pipe: can't establish more than one pipe at the same time for one peer")
		}
	}
	pl.awaiting.RUnlock()

	// stream is created here to ensure that pipe takes full control over stream
	out, err := pl.host.NewStream(ctx, peer, Protocol)
	if err != nil {
		return
	}

	defer func() {
		if err != nil {
			out.Reset()
		}
	}()

	err = writeProtoID(out, proto)
	if err != nil {
		return
	}

	inCh := pl.inboundFor(proto, peer)
	defer func() {
		if err != nil {
			pl.awaiting.Lock()
			pl.rmAwaiting(proto, peer)
			pl.awaiting.Unlock()
		}
	}()

	select {
	case in := <-inCh:
		return newPipe(proto, in, out), nil
	case <-time.After(HandshakeTimeout):
		return nil, ErrHandshake
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (pl *Pipeliner) Close() error {
	pl.host.RemoveStreamHandler(Protocol)
	return nil
}

func (pl *Pipeliner) handleStream(in network.Stream) {
	proto, err := readProtoID(in)
	if err != nil {
		log.Errorf("protocol negotiation failed: %s", err)
		in.Reset()
		return
	}

	pl.inboundFor(proto, in.Conn().RemotePeer()) <- in
}

func (pl *Pipeliner) inboundFor(proto protocol.ID, pid peer.ID) chan network.Stream {
	pl.awaiting.Lock()
	defer pl.awaiting.Unlock()

	peers, ok := pl.awaiting.m[proto]
	if !ok {
		peers = make(map[peer.ID]chan network.Stream, 1)
		pl.awaiting.m[proto] = peers
	}

	inCh, ok := peers[pid]
	if ok {
		pl.rmAwaiting(proto, pid)
	} else {
		inCh = make(chan network.Stream)
		peers[pid] = inCh
	}

	return inCh
}

// assumes lock is acquired
func (pl *Pipeliner) rmAwaiting(proto protocol.ID, peer peer.ID) {
	delete(pl.awaiting.m[proto], peer)
	if len(pl.awaiting.m[proto]) == 0 {
		delete(pl.awaiting.m, proto)
	}
}

func writeProtoID(w io.Writer, proto protocol.ID) error {
	err := binary.Write(w, binary.BigEndian, uint32(len(proto)))
	if err != nil {
		return err
	}

	_, err = w.Write([]byte(proto))
	return err
}

func readProtoID(r io.Reader) (protocol.ID, error) {
	var l uint32
	err := binary.Read(r, binary.BigEndian, &l)
	if err != nil {
		return "", err
	}

	proto := make([]byte, int(l))
	_, err = io.ReadFull(r, proto)
	return protocol.ID(proto), err
}
