# go-libp2p-pipe

Pipe is an effective way to reuse libp2p streams. While streams are 
lightweight there are still cases of protocols which needs a lot of messaging
between same peers for continuous time. Current libp2p flow suggests to create
new stream for every new message or request/response, what could be inefficient
in high flood of messages. Pipe suggests simple interface for two most common 
cases of stream messaging: simple message without any feedback and asynchronous 
request/response pattern.

Pipe is somewhere similar to request pipelining, but with one key difference -
requested host does not have to handle requests in line and can process 
them for long time, so responses could be sent at any time without any ordering. 

Pipe takes full control over stream and handles new stream creation on 
failures with graceful pipe closing on both sides of the pipe.

