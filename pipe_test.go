package pipe

import (
	"bytes"
	"context"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/protocol"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
)

var r = rand.New(rand.NewSource(time.Now().UnixNano()))

func TestPipeRequest(t *testing.T) {
	test := protocol.ID("test")
	ctx := context.Background()
	h1, h2 := buildHosts()
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

		msg.SendResponse(data)
	}, test)

	p, err := NewPipe(ctx, h2, h1.ID(), test)
	if err != nil {
		t.Fatal(err)
	}

	err = p.Send(msg)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := msg.GetResponse(ctx)
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
	h1, h2 := buildHosts()

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
	count := 10
	test := protocol.ID("test")
	h1, h2 := buildHosts()

	ph := func(p Pipe) {
		go func() {
			for i := 0; i < count; i++ {
				req, err := p.Next(ctx)
				if err != nil {
					t.Fatal(err)
				}

				go func(msg *Message) {
					<-time.After(time.Millisecond * time.Duration(r.Intn(2000)))
					msg.SendResponse(msg.Data())
				}(req)
			}
		}()

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

				resp, err := msg.GetResponse(ctx)
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
	b := make([]byte, 20)
	r.Read(b)
	return NewRequest(b)
}

func buildHosts() (host.Host, host.Host) {
	net, _ := mocknet.FullMeshConnected(context.TODO(), 2)
	return net.Hosts()[0], net.Hosts()[1]
}
