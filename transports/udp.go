package transports

import (
    "sync"
)

// NOTE:
// UDP packets here can be spoofed, so it is necessary to have the encryption/
// authentication layer working too.  There's no point in using a sequence
// number or something similar, since we don't make any guarantees about the
// delivery of packets (similar to the internet as a whole).
// TODO: protect against replay attacks somehow

const UDP_PORT = 44461

var udpClientMap = make(map[string]*genericPacketClient)
var udpClientMapLock sync.RWMutex

func NewUDPPacketClient(server string) (*genericPacketClient, error) {
    return newGenericPacketClient("udp", server, UDP_PORT, nil)
}

func NewUDPTransport(bindTo string) (*genericPacketTransport, error) {
    return newGenericPacketTransport("udp", bindTo, UDP_PORT, udpClientMap, udpClientMapLock)
}
