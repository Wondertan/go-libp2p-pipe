// Pipe is an effective way to reuse libp2p streams. While streams are
// lightweight, there are still cases of protocols which needs a lot of messaging
// between same peers for continuous time. Current libp2p flow suggests to create
// new stream for every new message or request/response, what could be inefficient
// and bandwidth wasteful in high flood of messages. Pipe suggests simple interface
// for two most common cases of messaging protocols over streams: simple message
// without any feedback and request/response pattern.
//
// Pipe is somewhere similar to request pipelining, but with one key difference -
// requested host does not have to handle requests in line and can process
// them for some time, so responses could be sent at any time without any ordering.
//
// Pipe takes full control over stream and handles new stream creation on
// failures with graceful pipe closing on both sides of the pipe.
package pipe

import (
	"context"
	"io"

	"github.com/libp2p/go-libp2p-core/protocol"
)

var (
	// Protocol ID
	Protocol protocol.ID = "/pipe/1.0.0"

	// MessageBufferSize specifies the size of buffer for incoming messages
	// If buffer is full, new messages will be dropped
	MessageBufferSize = 32
)

type Pipe interface {
	// Closes pipe for writing
	io.Closer

	// Send puts message in the pipe which transports it to other end.
	// Send is non-blocking
	Send(*Message) error

	// Next iteratively reads new messages from pipe
	Next(context.Context) (*Message, error)

	// Protocol returns protocol identifier defined in pipe
	Protocol() protocol.ID

	// Reset closes the pipe for reading and writing on both sides
	Reset() error
}

type Handler func(Pipe)
