# go-libp2p-pipe

[![Documentation](https://godoc.org/github.com/Wondertan/go-libp2p-pipe?status.svg)](https://godoc.org/github.com/Wondertan/go-libp2p-pipe)
[![Go Report Card](https://goreportcard.com/badge/github.com/Wondertan/go-libp2p-pipe)](https://goreportcard.com/report/github.com/Wondertan/go-libp2p-pipe)
[![License](https://img.shields.io/github/license/Wondertan/go-libp2p-pipe.svg?maxAge=2592000)](https://github.com/Wondertan/go-libp2p-pipe/blob/master/LICENSE)
[![standard-readme compliant](https://img.shields.io/badge/readme%20style-standard-brightgreen.svg?style=flat-square)](https://github.com/RichardLitt/standard-readme)

## Table of Contents

- [Background](#background)
- [Install](#install)
- [Usage](#usage)
  - [Establishing Pipe](#establishing-pipe)
  - [Sending/Receving Messages](#sendingreceving-messages)
  - [Sending/Receving Requests and Responses](#sendingreceving-requests-and-responses)
- [Maintainers](#maintainers)
- [Contributing](#contributing)
- [License](#license)

## Background

Pipe is an effective way to reuse libp2p streams. While streams are 
lightweight there are still cases of protocols which needs a lot of messaging
between same peers for continuous time. Current libp2p flow suggests to create
new stream for every new message or request/response, what could be inefficient
in high flood of messages and waste a lot of bandwidth. Pipe's aim is to fix the problems.

Pipe library suggests simple interface for two most common 
cases of bidirectional stream messaging: simple message without any feedback from remote peer and asynchronous 
request/response pattern. 

Pipe is somewhere similar to request pipelining, but with one key difference -
requested host does not have to handle requests in line and can process 
them for long time, so responses could be sent at any time without any ordering. 

Pipe takes full control over stream and handles new stream creation on 
failures with graceful pipe closing on both sides of the pipe.

## Install

`go get github.com/Wondertan/go-libp2p-pipe`

## Usage 

### Establishing Pipe
  ```go
  // register Pipe handler on remote Peer
  pipe.SetPipeHandler(host, pipeHandler, customProtocol)
  ```
  ```go
  // create new Pipe on own Peer
  pi, err := pipe.NewPipe(ctx, host, remotePeer, protocol)
  if err != nil {
    return err
  }
  ```
### Sending/Receving Messages
  ```go
  // create simple message with the data
  msg := pipe.NewMessage(bytes)

  // send message 
  err := pi.Send(msg)
  if err != nil {
    return err
  }
  ```
  ```go
  // dequeue received message from pipe
  msg := p.Next(context.Background())

  // retrieving sent data
  bytes := msg.Data()
  ```
### Sending/Receving Requests and Responses
  ```go
  // create new request message
  req := pipe.NewRequest(bytes)

  // send request
  err := pi.Send(req)
  if err != nil {
    return err
  }
  ```
  ```go
  // dequeue request
  req := p.Next(context.Background())

  // replying with the same data from the request
  req.Reply(req.Data())
  ```
  ```go
  // getting response with data from original requst
  data := req.Response(context.Background())
  ```
## Maintainers

[@Wondertan](https://github.com/Wondertan)

## Contributing

Feel free to dive in! [Open an issue](https://github.com/Wondertan/go-libp2p-pipe/issues/new) or submit PRs.

## License

[MIT](LICENSE) © Hlib Kanunnikov
