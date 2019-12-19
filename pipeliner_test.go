package pipe

import (
	"context"
	"testing"

	"github.com/libp2p/go-libp2p-core/protocol"
)

func TestPipeliner_NewPiper(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pls := pipeliners(ctx, 2)
	go func() {
		_, err := pls[1].NewPipe(ctx, protocol.TestingID, pls[0].host.ID())
		if err != nil {
			t.Fatal(err)
		}
	}()
	_, err := pls[0].NewPipe(ctx, protocol.TestingID, pls[1].host.ID())
	if err != nil {
		t.Fatal(err)
	}
}

func pipeliners(ctx context.Context, count int) []*Pipeliner {
	pls := make([]*Pipeliner, count)
	nodes, _ := buildHosts(ctx, 2)
	for i, n := range nodes {
		pls[i] = NewPipeLiner(n)
	}

	return pls
}
