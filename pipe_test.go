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
	test := protocol.ID("test")
	ctx := context.Background()
	msg := newRequest()
	testErr := errors.New("test_error")

	h1, h2, err := buildHosts(2)
	if err != nil {
		t.Fatal(err)
	}

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

	err = p.Send(msg)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := msg.Response(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(resp, msg.Data()) {
		t.Fatal("data is not equal")
	}

	err = p.Send(msg)
	if err != nil {
		t.Fatal(err)
	}

	_, err = msg.Response(ctx)
	if err.Error() != testErr.Error() {
		t.Fatal("error is wrong")
	}
}

func TestPipeMessage(t *testing.T) {
	test := protocol.ID("test")
	ctx := context.Background()

	h1, h2, err := buildHosts(2)
	if err != nil {
		t.Fatal(err)
	}

	msgIn := newMessage()

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
		t.Fatal("something wrong")
	}
}

func TestPipeClosing(t *testing.T) {
	test := protocol.ID("test")
	ctx := context.Background()
	h1, h2, err := buildHosts(2)
	if err != nil {
		t.Fatal(err)
	}

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

	req := newRequest()
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
		t.Fatal("is not properly closed")
	}
}

func BenchmarkPipeMessage(b *testing.B) {
	ctx := context.Background()
	test := protocol.ID("test")
	msgIn := newMessage()
	pch := make(chan Pipe)

	h1, h2, err := buildHosts(2)
	if err != nil {
		b.Fatal(err)
	}

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
	ctx := context.Background()
	test := protocol.ID("test")
	msgIn := newRequest()
	pch := make(chan Pipe)

	h1, h2, err := buildHosts(2)
	if err != nil {
		b.Fatal(err)
	}

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

// Test which sends multiple requests and responses from both pipe ends with delays
func TestPipeMultipleRequestResponses(t *testing.T) {
	ctx := context.Background()
	count := 50
	test := protocol.ID("test")
	h1, h2, err := buildHosts(2)
	if err != nil {
		t.Fatal(err)
	}

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

					err := msg.Reply(Data(msg.Data()))
					if err != nil {
						t.Fatal(err)
					}
				}(req)
			}
		}(p)

		wg := new(sync.WaitGroup)
		wg.Add(count)
		for i := 0; i < count; i++ {
			msg := newRequest()

			err := p.Send(msg)
			if err != nil {
				t.Fatal(err)
			}

			go func(msg *Message) {
				defer wg.Done()

				resp, err := msg.Response(ctx)
				if err != nil {
					t.Fatal(err)
				}

				if !bytes.Equal(resp, msg.Data()) {
					t.Fatal("something wrong")
				}
			}(msg)
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

func newMessage() *Message {
	l.Lock()
	defer l.Unlock()

	b := make([]byte, 100)
	r.Read(b)
	return NewMessage(b)
}

func newRequest() *Message {
	l.Lock()
	defer l.Unlock()

	b := make([]byte, 100)
	r.Read(b)
	return NewRequest(b)
}

func buildHosts(count int) (host.Host, host.Host, error) {
	hosts := make([]host.Host, count)
	for i := 0; i < count; i++ {
		hosts[i] = bhost.NewBlankHost(swarmt.GenSwarm(nil, context.TODO()))
	}

	for _, h1 := range hosts {
		for _, h2 := range hosts {
			if h1.ID() != h2.ID() {
				err := h1.Connect(context.TODO(), h2.Peerstore().PeerInfo(h2.ID()))
				if err != nil {
					return nil, nil, err
				}
			}
		}
	}

	return hosts[0], hosts[1], nil
}
