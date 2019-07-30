package pipe

import (
	"bytes"
	"context"
	"math/rand"
	"sync"
	"testing"
	"time"

	bhost "github.com/libp2p/go-libp2p-blankhost"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/protocol"
	swarmt "github.com/libp2p/go-libp2p-swarm/testing"
)

var r = rand.New(rand.NewSource(time.Now().UnixNano()))
var l sync.Mutex

func TestPipeRequest(t *testing.T) {
	test := protocol.ID("test")
	ctx := context.Background()
	h1, h2 := buildHosts(t)
	msg := newRequest()

	SetPipeHandler(h1, func(p Pipe) {
		req, err := p.Next(ctx)
		if err != nil {
			t.Fatal(err)
		}

		data := req.Data()
		if !bytes.Equal(data, msg.Data()) {
			t.Fatal("something wrong")
		}

		msg.Reply(data)
	}, test)

	p, err := NewPipe(ctx, h2, h1.ID(), test)
	if err != nil {
		t.Fatal(err)
	}

	err = p.Send(msg)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := msg.Response(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(resp, msg.Data()) {
		t.Fatal("something wrong")
	}
}

func TestPipeMessage(t *testing.T) {
	test := protocol.ID("test")
	ctx := context.Background()
	h1, h2 := buildHosts(t)

	b := make([]byte, 20)
	r.Read(b)

	SetPipeHandler(h1, func(p Pipe) {
		msg := NewMessage(b)

		err := p.Send(msg)
		if err != nil {
			t.Fatal(err)
		}
	}, test)

	p, err := NewPipe(ctx, h2, h1.ID(), test)
	if err != nil {
		t.Fatal(err)
	}

	msg, err := p.Next(ctx)
	if err != nil {
		t.Fatal(err)
	}

	data := msg.Data()
	if !bytes.Equal(data, b) {
		t.Fatal("something wrong")
	}

	if !bytes.Equal(data, b) {
		t.Fatal("something wrong")
	}
}

func TestMultipleAsyncResponses(t *testing.T) {
	ctx := context.Background()
	count := 100
	test := protocol.ID("test")
	h1, h2 := buildHosts(t)

	ph := func(p Pipe) {
		go func(p Pipe) {
			for i := 0; i < count; i++ {
				req, err := p.Next(ctx)
				if err != nil {
					t.Fatal(err)
				}

				go func(msg *Message) {
					l.Lock()
					rn := r.Intn(200)
					l.Unlock()

					<-time.After(time.Millisecond * time.Duration(rn))

					msg.Reply(msg.Data())
				}(req)
			}
		}(p)

		msgs := make([]*Message, count)
		for i := 0; i < count; i++ {
			msgs[i] = newRequest()

			err := p.Send(msgs[i])
			if err != nil {
				t.Fatal(err)
			}
		}

		wg := new(sync.WaitGroup)
		wg.Add(count)
		for i := 0; i < count; i++ {
			go func(msg *Message) {
				defer wg.Done()

				resp, err := msg.Response(ctx)
				if err != nil {
					t.Fatal(err)
				}

				if !bytes.Equal(resp, msg.Data()) {
					t.Fatal("something wrong")
				}
			}(msgs[i])
		}

		wg.Wait()
	}

	SetPipeHandler(h1, ph, test)

	p, err := NewPipe(ctx, h2, h1.ID(), test)
	if err != nil {
		t.Fatal(err)
	}

	ph(p)
}

func newRequest() *Message {
	l.Lock()
	defer l.Unlock()

	b := make([]byte, 100)
	r.Read(b)
	return NewRequest(b)
}

func buildHosts(t *testing.T) (host.Host, host.Host) {
	count := 2
	hosts := make([]host.Host, count)
	for i := 0; i < count; i++ {
		hosts[i] = bhost.NewBlankHost(swarmt.GenSwarm(t, context.TODO()))
	}

	for _, h1 := range hosts {
		for _, h2 := range hosts {
			if h1.ID() != h2.ID() {
				err := h1.Connect(context.TODO(), h2.Peerstore().PeerInfo(h2.ID()))
				if err != nil {
					t.Fatal(err)
				}
			}
		}
	}

	return hosts[0], hosts[1]
}
