package transports

// This interface represents a single connected client.
type PacketClient interface {
    SendChannel() chan []byte
    RecvChannel() chan []byte
    Close()
    IsReliable() bool
    Describe() string
}

type Transport interface {
    AcceptChannel() chan PacketClient
}
