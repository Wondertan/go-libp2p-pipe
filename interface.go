package pipe

import (
	"context"
	"io"

	"github.com/libp2p/go-libp2p-core/protocol"
)

var Protocol protocol.ID = "/pipe/1.0.0"

type Pipe interface {
	io.Closer

	Send(*Message) error
	Next(context.Context) (*Message, error)
}

type Handler func(Pipe)
