package emdr

import (
	"runtime/debug"

	"github.com/pebbe/zmq4"
)

var messageChannel chan []byte
var upstreamSocket *zmq4.Socket

// Initialize sets up the EMDR emulation socket
func Initialize() chan<- []byte {
	messageChannel = make(chan []byte, 100)

	s, err := zmq4.NewSocket(zmq4.PUB)
	if err != nil {
		panic(err)
	}

	upstreamSocket = s

	upstreamSocket.Bind("tcp://*:8050")

	go runSendLoop()

	return messageChannel
}

func runSendLoop() {
	defer upstreamSocket.Close()

	for {
		msg := <-messageChannel
		upstreamSocket.SendBytes(msg, 0)

		// Don't block OS memory for that long due to a bigger message every now and then
		debug.FreeOSMemory()
	}
}
