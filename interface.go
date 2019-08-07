package pipe

import (
	"context"
	"github.com/libp2p/go-libp2p-core/network"
	"io"

	"github.com/libp2p/go-libp2p-core/protocol"
)

var Protocol protocol.ID = "/pipe/1.0.0"

var (
	// MaxWriteAttempts specifies amount of retries to write on failure
	MaxWriteAttempts = 3

	// MessageBuffer specifies the size of buffer for incoming messages
	// If buffer is full, new messages will be dropped
	MessageBuffer = 8
)

type Pipe interface {
	// Closes pipe for writing
	io.Closer

	// Send puts message in the pipe which after are transported to other pipe's end
	Send(*Message) error

	// Next iteratively reads new messages from pipe
	Next(context.Context) (*Message, error)

	// Protocol returns protocol identifier defined in pipe
	Protocol() protocol.ID

	// Conn returns underlying connection used by pipe
	Conn() network.Conn

	// Reset closes the pipe for reading and writing on both sides
	Reset() error
}

type Handler func(Pipe)
