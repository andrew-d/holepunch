package transports

// This interface represents a single connected client.
type PacketClient interface {
    // Send a single packet, error if necessary.  Will block for some time if
    // no packets can currently be sent - e.g. if the send window is full on
    // some transports.
    SendPacket(pkt []byte) error

    // Get the channel to use to receive packets.  This is used, rather than
    // a simple "read" function, since it allows us to use the select{}
    // primitive to do multiple things in one loop.  Also, we apparently
    // can't have a variable requirement in an interface, so we just have this
    // function that returns the underlying variable.
    PacketChannel() chan []byte

    // Close this transport down.
    Close()

    // This can return whatever - mostly used for helpful debugging.
    Describe() string
}

// This interface represents a server - something that will accept clients.
type GenericServer interface {
    // Accept a single client and return it.
    AcceptChannel() chan PacketClient
}
