package transports

// This interface represents a single connected client.
type PacketClient interface {
    SendChannel() chan []byte
    RecvChannel() chan []byte
    Close()
    Describe() string
}

type Transport interface {
    AcceptChannel() chan PacketClient
}
