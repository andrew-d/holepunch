package transports

import (
    "fmt"
    "log"
    "net"
    "sync"
)

type genericPacketClient struct {
    send_ch chan []byte
    recv_ch chan []byte
    conn    net.Conn
    addr    net.Addr

    host string
    net  string

    onClose func()
}

type packetMessage struct {
    msg  []byte
    addr net.Addr
}

// TODO: lock?

func newGenericPacketClient(network, server string, port uint16,
    onClose func()) (*genericPacketClient, error) {

    host := fmt.Sprintf("%s:%d", server, port)

    conn, err := net.Dial(network, host)
    if err != nil {
        return nil, err
    }

    addr := conn.RemoteAddr()
    send_ch := make(chan []byte)
    recv_ch := make(chan []byte)

    ret := &genericPacketClient{send_ch, recv_ch, conn, addr, host, network, onClose}
    ret.startAsClientConn()

    return ret, nil
}

func (p *genericPacketClient) startAsClientConn() {
    go p.startSendLoop()
    go p.startRecvLoop()
}

func (p *genericPacketClient) startAsServerConn(forward_to chan packetMessage) {
    go p.startForwardLoop(forward_to)
}

func (p *genericPacketClient) startForwardLoop(forward_to chan packetMessage) {
    var pkt []byte
    var msg packetMessage

    // TODO: some way to stop this
    for {
        pkt = <-p.send_ch
        msg.msg = pkt
        msg.addr = p.addr
        forward_to <- msg
    }
}

func (p *genericPacketClient) startSendLoop() {
    var pkt []byte
    var err error

    // TODO: some way to stop this
    for {
        pkt = <-p.send_ch

        log.Printf("Writing packet of length %d...\n", len(pkt))
        _, err = p.conn.Write(pkt)
        if err != nil {
            log.Printf("Error writing packet: %s\n", err)
            break
        }
    }
}

func (p *genericPacketClient) startRecvLoop() {
    var pkt [65535]byte
    var err error
    var n int
    var addr net.Addr

    pconn := p.conn.(net.PacketConn)

    // TODO: some way to stop this
    for {
        n, addr, err = pconn.ReadFrom(pkt[0:])
        if err != nil {
            log.Printf("Error reading packet: %s\n", err)
            break
        }

        if addr.String() != p.addr.String() {
            log.Printf("Addresses not equal: %s != %s\n", addr.String(), p.addr.String())
            continue
        }

        log.Printf("Received packet of length %d\n", n)
        p.recv_ch <- pkt[0:n]
    }
}

func (p *genericPacketClient) SendChannel() chan []byte {
    return p.send_ch
}

func (p *genericPacketClient) RecvChannel() chan []byte {
    return p.recv_ch
}

func (p *genericPacketClient) Close() {
    p.conn.Close()

    if p.onClose != nil {
        p.onClose()
    }
}

func (p *genericPacketClient) IsReliable() bool {
    return false
}

func (p *genericPacketClient) Describe() string {
    return fmt.Sprintf("genericPacketClient(%s)", p.net)
}

// --------------------------------------------------------------------------------

type genericPacketTransport struct {
    conn      net.PacketConn
    accept_ch chan PacketClient
    send_ch   chan packetMessage
    network   string
}

func (p *genericPacketTransport) AcceptChannel() chan PacketClient {
    return p.accept_ch
}

func newGenericPacketTransport(network, bindTo string, port uint16,
    clientMap map[string]*genericPacketClient,
    clientMapLock sync.RWMutex) (*genericPacketTransport, error) {

    host := fmt.Sprintf("%s:%d", bindTo, port)

    conn, err := net.ListenPacket(network, host)
    if err != nil {
        return nil, err
    }

    accept_ch := make(chan PacketClient)
    send_ch := make(chan packetMessage)
    ret := &genericPacketTransport{conn, accept_ch, send_ch, network}

    go ret.sendPackets()
    go ret.acceptConnections(clientMap, clientMapLock)

    return ret, nil
}

func (p *genericPacketTransport) sendPackets() {
    var msg packetMessage
    var err error

    // TODO: some way to stop this
    for {
        msg = <-p.send_ch

        log.Printf("Writing packet of length %d...\n", len(msg.msg))
        _, err = p.conn.WriteTo(msg.msg, msg.addr)
        if err != nil {
            log.Printf("Error writing packet: %s\n", err)
            break
        }
    }
}

func (p *genericPacketTransport) acceptConnections(clientMap map[string]*genericPacketClient,
    clientMapLock sync.RWMutex) {

    log.Println("Started accepting clients")

    var pkt [65535]byte
    var n int
    var err error
    var addr net.Addr

    for {
        n, addr, err = p.conn.ReadFrom(pkt[:])
        if err != nil {
            log.Printf("Error reading new packet: %s\n")
        }
        log.Printf("Got packet of length %d\n", n)

        clientMapLock.RLock()
        client, found := clientMap[addr.String()]
        clientMapLock.RUnlock()

        if !found {
            log.Printf("Got new client: %s\n", addr)

            // We create a new client that will proxy all writes to this
            // transport's underlying send channel.
            recv_ch := make(chan []byte)
            send_ch := make(chan []byte)

            // When the client is closed, it needs to remove itself from the map.
            onClose := func () {
                clientMapLock.Lock()
                delete(clientMap, addr.String())
                clientMapLock.Unlock()
            }

            client = &genericPacketClient{
                send_ch, recv_ch,
                nil, addr,
                "host", p.network,
                onClose,
            }
            go client.startAsServerConn(p.send_ch)

            clientMapLock.Lock()
            clientMap[addr.String()] = client
            clientMapLock.Unlock()

            p.accept_ch <- client
        }

        client.RecvChannel() <- pkt[0:n]
    }
}
