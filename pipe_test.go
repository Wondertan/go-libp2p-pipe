package pipe

import (
	"bytes"
	"context"
	"errors"
	"math/rand"
	"sync"
	"testing"
	"time"

	bhost "github.com/libp2p/go-libp2p-blankhost"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/protocol"
	swarmt "github.com/libp2p/go-libp2p-swarm/testing"
)

func TestPipeRequestResponse(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	test := protocol.ID("test")
	req := newRandRequest()
	testErr := errors.New("test_error")

	h, err := buildHosts(ctx, 2)
	if err != nil {
		t.Fatal(err)
	}
	h1, h2 := h[0], h[1]

	SetPipeHandler(h1, func(p Pipe) {
		req, err := p.Next(ctx)
		if err != nil {
			t.Fatal(err)
		}

		err = req.Reply(Data(req.Data()))
		if err != nil {
			t.Fatal(err)
		}

		req, err = p.Next(ctx)
		if err != nil {
			t.Fatal(err)
		}

		err = req.Reply(Error(testErr))
		if err != nil {
			t.Fatal(err)
		}
	}, test)

	p, err := NewPipe(ctx, h2, h1.ID(), test)
	if err != nil {
		t.Fatal(err)
	}

	err = p.Send(req)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := req.Response(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(resp, req.Data()) {
		t.Fatal("req is not equal with resp")
	}

	err = p.Send(req)
	if err != nil {
		t.Fatal(err)
	}

	_, err = req.Response(ctx)
	if err.Error() != testErr.Error() {
		t.Fatal("error is wrong")
	}
}

func TestPipeMessage(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	test := protocol.ID("test")
	h, err := buildHosts(ctx, 2)
	if err != nil {
		t.Fatal(err)
	}
	h1, h2 := h[0], h[1]

	msgIn := newRandMessage()

	SetPipeHandler(h1, func(p Pipe) {
		err := p.Send(msgIn)
		if err != nil {
			t.Fatal(err)
		}
	}, test)

	p, err := NewPipe(ctx, h2, h1.ID(), test)
	if err != nil {
		t.Fatal(err)
	}

	msgOut, err := p.Next(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(msgOut.Data(), msgIn.Data()) {
		t.Fatal("messages are not equal")
	}
}

func TestPipeClosing(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	test := protocol.ID("test")
	h, err := buildHosts(ctx, 2)
	if err != nil {
		t.Fatal(err)
	}
	h1, h2 := h[0], h[1]

	SetPipeHandler(h1, func(p Pipe) {
		req, err := p.Next(ctx)
		if err != nil {
			t.Fatal(err)
		}

		err = req.Reply(Data(req.Data()))
		if err != nil {
			t.Fatal(err)
		}

		err = p.Send(NewMessage(req.Data()))
		if err != nil {
			t.Fatal(err)
		}

		err = p.Close()
		if err != nil {
			t.Fatal(err)
		}
	}, test)

	p, err := NewPipe(ctx, h2, h1.ID(), test)
	if err != nil {
		t.Fatal(err)
	}

	req := newRandRequest()
	err = p.Send(req)
	if err != nil {
		t.Fatal(err)
	}

	err = p.Close()
	if err != nil {
		t.Fatal(err)
	}

	_, err = req.Response(ctx)
	if err != nil {
		t.Fatal(err)
	}

	_, err = p.Next(ctx)
	if err != nil {
		t.Fatal(err)
	}

	err = p.Send(req)
	if err != ErrClosed {
		t.Fatal("pipe is not properly closed")
	}
}

func BenchmarkPipeMessage(b *testing.B) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	test := protocol.ID("test")
	msgIn := newRandMessage()
	pch := make(chan Pipe)

	h, err := buildHosts(ctx, 2)
	if err != nil {
		b.Fatal(err)
	}
	h1, h2 := h[0], h[1]

	SetPipeHandler(h1, func(p Pipe) {
		pch <- p
	}, test)

	p1, err := NewPipe(ctx, h2, h1.ID(), test)
	if err != nil {
		b.Fatal(err)
	}

	p2 := <-pch

	b.ResetTimer()
	b.ReportAllocs()
	for n := 0; n < b.N; n++ {
		err = p1.Send(msgIn)
		if err != nil {
			b.Fatal(err)
		}

		_, err := p2.Next(ctx)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPipeRequestResponse(b *testing.B) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	test := protocol.ID("test")
	msgIn := newRandRequest()
	pch := make(chan Pipe)

	h, err := buildHosts(ctx, 2)
	if err != nil {
		b.Fatal(err)
	}
	h1, h2 := h[0], h[1]

	SetPipeHandler(h1, func(p Pipe) {
		pch <- p
	}, test)

	p1, err := NewPipe(ctx, h2, h1.ID(), test)
	if err != nil {
		b.Fatal(err)
	}

	p2 := <-pch

	b.ResetTimer()
	b.ReportAllocs()
	for n := 0; n < b.N; n++ {
		err = p1.Send(msgIn)
		if err != nil {
			b.Fatal(err)
		}

		msgOut, err := p2.Next(ctx)
		if err != nil {
			b.Fatal(err)
		}

		err = msgOut.Reply(Data(msgOut.Data()))
		if err != nil {
			b.Fatal(err)
		}

		_, err = msgIn.Response(ctx)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func TestPipeMultipleRequestResponses(t *testing.T) {
	messagesCount := 5000
	maxReplyDelay := time.Millisecond * 200

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	test := protocol.ID("test")
	h, err := buildHosts(ctx, 2)
	if err != nil {
		t.Fatal(err)
	}
	h1, h2 := h[0], h[1]

	ph := func(p Pipe) {
		defer p.Close()

		wg := new(sync.WaitGroup)

		go func(p Pipe) {
			for i := 0; i < messagesCount; i++ {
				req, err := p.Next(ctx)
				if err != nil {
					t.Fatal(err)
				}

				wg.Add(1)
				go func(req *Message) {
					defer wg.Done()

					delay(ctx, maxReplyDelay)

					err := req.Reply(Data(req.Data()))
					if err != nil {
						t.Fatal(err)
					}
				}(req)
			}
		}(p)

		for i := 0; i < messagesCount; i++ {
			req := newRandRequest()

			err := p.Send(req)
			if err != nil {
				t.Fatal(err)
			}

			wg.Add(1)
			go func(req *Message) {
				defer wg.Done()

				resp, err := req.Response(ctx)
				if err != nil {
					t.Fatal(err)
				}

				if !bytes.Equal(resp, req.Data()) {
					t.Fatal("req is not equal with the resp")
				}
			}(req)
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

var r = rand.New(rand.NewSource(time.Now().UnixNano()))
var l sync.Mutex

func newRandMessage() *Message {
	l.Lock()
	defer l.Unlock()

	b := make([]byte, 100)
	r.Read(b)
	return NewMessage(b)
}

func newRandRequest() *Message {
	l.Lock()
	defer l.Unlock()

	b := make([]byte, 100)
	r.Read(b)
	return NewRequest(b)
}

func delay(ctx context.Context, max time.Duration) {
	l.Lock()
	rn := r.Intn(int(max))
	l.Unlock()

	select {
	case <-time.After(time.Duration(rn)):
		return
	case <-ctx.Done():
		return
	}
}

func buildHosts(ctx context.Context, count int) ([]host.Host, error) {
	hosts := make([]host.Host, count)
	for i := 0; i < count; i++ {
		hosts[i] = bhost.NewBlankHost(swarmt.GenSwarm(nil, ctx))
	}

	for _, h1 := range hosts {
		for _, h2 := range hosts {
			if h1.ID() != h2.ID() {
				err := h1.Connect(ctx, h2.Peerstore().PeerInfo(h2.ID()))
				if err != nil {
					return nil, err
				}
			}
		}
	}

	return hosts, nil
}
