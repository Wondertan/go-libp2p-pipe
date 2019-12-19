package pipe

import (
	"context"
	"errors"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p-core/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPipeRequestResponse(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	testErr := errors.New("test_error")
	req := newRandRequest()
	p1, p2 := twoPipes(t, ctx)

	go func() {
		req, err := p1.Next(ctx)
		require.Nil(t, err, err)

		err = req.Reply(Data(req.Data()))
		require.Nil(t, err, err)

		req, err = p1.Next(ctx)
		require.Nil(t, err, err)

		err = req.Reply(Error(testErr))
		require.Nil(t, err, err)
	}()

	err := p2.Send(req)
	require.Nil(t, err, err)

	resp, err := req.Response(ctx)
	require.Nil(t, err, err)
	assert.Equal(t, resp, req.Data(), "req is not equal with resp")

	err = p2.Send(req)
	require.Nil(t, err, err)

	resp, err = req.Response(ctx)
	assert.Nil(t, resp)
	assert.Equal(t, testErr.Error(), err.Error(), "error is wrong")
}

func TestPipeMessage(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	msgIn := newRandMessage()
	p1, p2 := twoPipes(t, ctx)

	go func() {
		err := p1.Send(msgIn)
		require.Nil(t, err, err)
	}()

	msgOut, err := p2.Next(ctx)
	require.Nil(t, err, err)
	assert.Equal(t, msgOut.Data(), msgIn.Data(), "messages are not equal")
}

func TestPipeClosing(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	req := newRandRequest()
	p1, p2 := twoPipes(t, ctx)

	go func() {
		req, err := p1.Next(ctx)
		require.Nil(t, err, err)

		err = req.Reply(Data(req.Data()))
		require.Nil(t, err, err)

		err = p1.Send(NewMessage(req.Data()))
		require.Nil(t, err, err)

		err = p1.Close()
		require.Nil(t, err, err)
	}()

	err := p2.Send(req)
	require.Nil(t, err, err)

	err = p2.Close()
	require.Nil(t, err, err)

	_, err = req.Response(ctx)
	require.Nil(t, err, err)

	_, err = p2.Next(ctx)
	require.Nil(t, err, err)

	err = p2.Send(req)
	assert.Equal(t, err, ErrClosed, "pipe is not properly closed")
}

func BenchmarkPipeMessage(b *testing.B) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	msgIn := newRandRequest()
	p1, p2 := twoPipes(b, ctx)

	b.ResetTimer()
	b.ReportAllocs()
	for n := 0; n < b.N; n++ {
		err := p1.Send(msgIn)
		require.Nil(b, err, err)

		msgOut, err := p2.Next(ctx)
		require.Nil(b, err, err)
		assert.NotNil(b, msgOut)
	}
}

func BenchmarkPipeRequestResponse(b *testing.B) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	msgIn := newRandRequest()
	p1, p2 := twoPipes(b, ctx)

	b.ResetTimer()
	b.ReportAllocs()
	for n := 0; n < b.N; n++ {
		err := p1.Send(msgIn)
		require.Nil(b, err, err)

		msgOut, err := p2.Next(ctx)
		require.Nil(b, err, err)

		err = msgOut.Reply(Data(msgOut.Data()))
		require.Nil(b, err, err)

		data, err := msgIn.Response(ctx)
		require.Nil(b, err, err)
		assert.NotNil(b, data)
	}
}

func TestPipeMultipleRequestResponses(t *testing.T) {
	messagesCount := 5000
	maxReplyDelay := time.Millisecond * 200

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	p1, p2 := twoPipes(t, ctx)
	ph := func(p Pipe) {
		wg := new(sync.WaitGroup)

		go func(p Pipe) {
			for i := 0; i < messagesCount; i++ {
				req, err := p.Next(ctx)
				require.Nil(t, err, err)

				wg.Add(1)
				go func(req *Message) {
					defer wg.Done()

					delay(ctx, maxReplyDelay)

					err := req.Reply(Data(req.Data()))
					require.Nil(t, err, err)
				}(req)
			}
		}(p)

		for i := 0; i < messagesCount; i++ {
			req := newRandRequest()

			err := p.Send(req)
			require.Nil(t, err, err)

			wg.Add(1)
			go func(req *Message) {
				defer wg.Done()

				resp, err := req.Response(ctx)
				require.Nil(t, err, err)
				assert.Equal(t, resp, req.Data())
			}(req)
		}

		wg.Wait()
	}

	go ph(p2)
	ph(p1)
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
	rd := time.Duration(r.Intn(int(max)))
	l.Unlock()

	select {
	case <-time.After(rd):
		return
	case <-ctx.Done():
		return
	}
}

func twoPipes(t fataler, ctx context.Context) (*pipe, *pipe) {
	pl1, pl2 := twoPipeliners(t, ctx)

	pCh := make(chan *pipe)
	go func() {
		p, err := pl1.NewPipe(ctx, protocol.TestingID, pl2.host.ID())
		if err != nil {
			t.Fatal(err)
		}
		pCh <- p.(*pipe)
	}()

	p, err := pl2.NewPipe(ctx, protocol.TestingID, pl1.host.ID())
	if err != nil {
		t.Fatal(err)
	}

	return p.(*pipe), <-pCh
}
