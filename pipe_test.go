package pipe

import (
	"bytes"
	"context"
	"errors"
	"math/rand"
	"os"
	"sync"
	"testing"
	"time"

	logging "github.com/ipfs/go-log"
	lwriter "github.com/ipfs/go-log/writer"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/protocol"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
)

func TestPipeRequestResponse(t *testing.T) {
	ctx := context.Background()
	// ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	// defer cancel()

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

		err = req.Reply(ctx, Data(req.Data()))
		if err != nil {
			t.Fatal(err)
		}

		req, err = p.Next(ctx)
		if err != nil {
			t.Fatal(err)
		}

		err = req.Reply(ctx, Error(testErr))
		if err != nil {
			t.Fatal(err)
		}
	}, test)

	p, err := NewPipe(ctx, h2, h1.ID(), test)
	if err != nil {
		t.Fatal(err)
	}

	err = p.Send(ctx, req)
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

	err = p.Send(ctx, req)
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
		err := p.Send(ctx, msgIn)
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

		err = req.Reply(ctx, Data(req.Data()))
		if err != nil {
			t.Fatal(err)
		}

		err = p.Send(ctx, NewMessage(req.Data()))
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
	err = p.Send(ctx, req)
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

	err = p.Send(ctx, req)
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
		err = p1.Send(ctx, msgIn)
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
		err = p1.Send(ctx, msgIn)
		if err != nil {
			b.Fatal(err)
		}

		msgOut, err := p2.Next(ctx)
		if err != nil {
			b.Fatal(err)
		}

		err = msgOut.Reply(ctx, Data(msgOut.Data()))
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
	logging.SetLogLevel("pipe", "debug")
	lwriter.WriterGroup.AddWriter(os.Stderr)

	messagesCount := 50
	maxReplyDelay := time.Millisecond * 200

	test := protocol.ID("test")
	ctx := context.Background()

	h, err := buildHosts(ctx, 2)
	if err != nil {
		t.Fatal(err)
	}
	h1, h2 := h[0], h[1]

	var count int32
	ph := func(p Pipe) {
		ctx := log.Start(ctx, "PipeMultiple")
		defer log.Finish(ctx)
		log.SetTag(ctx, "pipe", count)

		go func(p Pipe) {
			wg := new(sync.WaitGroup)
			wg.Add(messagesCount)
			for i := 0; i < messagesCount; i++ {
				req, err := p.Next(ctx)
				if err != nil {
					t.Fatal(err)
				}

				go func(req *Message) {
					defer wg.Done()

					delay(ctx, maxReplyDelay)

					err := req.Reply(ctx, Data(req.Data()))
					if err != nil {
						t.Fatal(err)
					}
				}(req)
			}

			wg.Wait()

			err = p.Close()
			if err != nil {
				t.Fatal(err)
			}
		}(p)

		wg := new(sync.WaitGroup)
		wg.Add(messagesCount)
		for i := 0; i < messagesCount; i++ {
			req := newRandRequest()

			err := p.Send(ctx, req)
			if err != nil {
				t.Fatal(err)
			}

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
	net, _ := mocknet.FullMeshConnected(ctx, count)
	return net.Hosts(), nil
}
