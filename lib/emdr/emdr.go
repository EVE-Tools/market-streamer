package emdr

import (
	"github.com/pebbe/zmq4"
)

var messageChannel chan []byte
var upstreamSocket *zmq4.Socket

// Initialize sets up the EMDR emulation socket
func Initialize(bindEndpoint string) chan<- []byte {
	messageChannel = make(chan []byte, 100)

	s, err := zmq4.NewSocket(zmq4.PUB)
	if err != nil {
		panic(err)
	}

	upstreamSocket = s

	upstreamSocket.Bind(bindEndpoint)

	go runSendLoop()

	return messageChannel
}

func runSendLoop() {
	defer upstreamSocket.Close()

	for {
		msg := <-messageChannel
		upstreamSocket.SendBytes(msg, 0)
	}
}
