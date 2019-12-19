package pipe

import (
	"context"
	"testing"

	"github.com/libp2p/go-libp2p-core/protocol"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/stretchr/testify/assert"
)

func TestPipeliner_NewPiper(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pl1, pl2 := twoPipeliners(t, ctx)
	go func() {
		p, err := pl1.NewPipe(ctx, protocol.TestingID, pl2.host.ID())
		assert.Nil(t, err, err)
		assert.NotNil(t, p)
	}()

	p, err := pl2.NewPipe(ctx, protocol.TestingID, pl1.host.ID())
	assert.Nil(t, err, err)
	assert.NotNil(t, p)
}

func twoPipeliners(t fataler, ctx context.Context) (*Pipeliner, *Pipeliner) {
	pls := pipeliners(t, ctx, 2)
	return pls[0], pls[1]
}

func pipeliners(t fataler, ctx context.Context, count int) []*Pipeliner {
	net, err := mocknet.FullMeshConnected(ctx, count)
	if err != nil {
		t.Fatal(err)
	}

	pls := make([]*Pipeliner, count)
	for i, n := range net.Hosts() {
		pls[i] = NewPipeLiner(n)
	}

	return pls
}

type fataler interface {
	Fatal(args ...interface{})
}
