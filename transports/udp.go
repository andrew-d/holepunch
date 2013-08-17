package transports

import (
    "fmt"
    "log"
    "net"
    "sync"
    "time"
)

// NOTE:
// UDP packets here can be spoofed, so it is necessary to have the encryption/
// authentication layer working too.  There's no point in using a sequence
// number or something similar, since we don't make any guarantees about the
// delivery of packets (similar to the internet as a whole).

/*
- Client creates new connection with DialUDP, simply reads/writes to the socket
- Server creates new socket, all connection read/writes are proxied to the
  server's channel, which does the actual reads/writes


*/

type UDPPacketClient struct {
    host    string
    send_ch chan []byte
    recv_ch chan []byte
    conn    *net.UDPConn
    addr    *net.UDPAddr
}

type udpMessage struct {
    msg  []byte
    addr *net.UDPAddr
}

const UDP_PORT = 44461

var clientMap = make(map[string]*UDPPacketClient)
var clientMapLock sync.RWMutex

func NewUDPPacketClient(server string) (*UDPPacketClient, error) {
    host := fmt.Sprintf("%s:%d", server, UDP_PORT)

    addr, err := net.ResolveUDPAddr("udp", host)
    if err != nil {
        log.Printf("Error connecting with UDP: %s", err)
        return nil, err
    }

    conn, err := net.DialUDP("udp", nil, addr)
    if err != nil {
        log.Printf("Error connecting with UDP: %s", err)
        return nil, err
    }

    send_ch := make(chan []byte)
    recv_ch := make(chan []byte)
    ret := &UDPPacketClient{host, send_ch, recv_ch, conn, addr}

    // For a client, the sends/receives are actually performed on the
    // connection.
    go ret.doSend()
    go ret.doRecv()

    return ret, nil
}

func (u *UDPPacketClient) doSend() {
    var pkt []byte
    var err error

    // TODO: some way to stop this
    for {
        pkt = <-u.send_ch

        log.Printf("Writing packet of length %d...\n", len(pkt))
        _, err = u.conn.Write(pkt)
        if err != nil {
            log.Printf("Error writing packet: %s\n", err)
            break
        }
    }
}

func (u *UDPPacketClient) doForwardSend(forward_to chan udpMessage) {
    var pkt []byte
    var msg udpMessage

    // TODO: some way to stop this
    for {
        pkt = <-u.send_ch
        msg.msg = pkt
        msg.addr = u.addr
        forward_to <- msg
    }
}

func (u *UDPPacketClient) doRecv() {
    var pkt [65535]byte
    var err error
    var n int
    var addr *net.UDPAddr

    for {
        n, addr, err = u.conn.ReadFromUDP(pkt[0:])
        if err != nil {
            log.Printf("Error reading packet: %s\n", err)
            break
        }

        if n == 0 {
            time.Sleep(100 * time.Millisecond)
            continue
        }

        if !addr.IP.Equal(u.addr.IP) {
            log.Printf("IPs not equal: %s != %s\n", addr.IP, u.addr.IP)
            continue
        }

        if addr.Port != u.addr.Port {
            log.Printf("Ports not equal: %d != %d\n", addr.Port, u.addr.Port)
            continue
        }

        log.Printf("Received packet of length %d\n", n)
        u.recv_ch <- pkt[0:n]
    }
}

func (u *UDPPacketClient) SendChannel() chan []byte {
    return u.send_ch
}

func (u *UDPPacketClient) RecvChannel() chan []byte {
    return u.recv_ch
}

func (u *UDPPacketClient) Close() {
    u.conn.Close()

    clientMapLock.Lock()
    delete(clientMap, u.addr.String())
    clientMapLock.Unlock()
}

func (u *UDPPacketClient) IsReliable() bool {
    return false
}

func (u *UDPPacketClient) Describe() string {
    return "UDPPacketClient"
}

type UDPTransport struct {
    listen    *net.UDPConn
    accept_ch chan PacketClient
    send_ch   chan udpMessage
}

func (u *UDPTransport) AcceptChannel() chan PacketClient {
    return u.accept_ch
}

func NewUDPTransport(bindTo string) (*UDPTransport, error) {
    host := fmt.Sprintf("%s:%d", bindTo, TCP_PORT)

    saddr, err := net.ResolveUDPAddr("udp", host)
    if err != nil {
        return nil, err
    }

    listener, err := net.ListenUDP("udp", saddr)
    if err != nil {
        return nil, err
    }

    client_ch := make(chan PacketClient)
    send_ch := make(chan udpMessage)
    trans := &UDPTransport{listener, client_ch, send_ch}

    go trans.acceptConnections()
    go trans.sendPackets()

    return trans, nil
}

func (u *UDPTransport) acceptConnections() {
    log.Println("Started accepting clients")

    // UDP doesn't really have a concept of "clients", like TCP does.  Instead,
    // we will do the following for each incoming UDP packet:
    //  - Check if the remote address is in our map of address -> connection.
    //    If so, we dispatch to that connection.  If not, continue.
    //  - Create a new connection, insert it into the map, and then start it.
    //  - Whenever a connection is closed, we remove it from the map.

    var pkt [65535]byte
    var n int
    var err error
    var addr *net.UDPAddr

    for {
        n, addr, err = u.listen.ReadFromUDP(pkt[:])
        if err != nil {
            log.Printf("Error reading new packet: %s\n")
        }
        log.Printf("Got packet of length %d\n", n)

        clientMapLock.RLock()
        client, found := clientMap[addr.String()]
        clientMapLock.RUnlock()

        if !found {
            log.Printf("Got new client: %s\n", addr)

            // We create a new UDP client that will proxy all writes to this
            // transport's underlying send channel.
            recv_ch := make(chan []byte)
            send_ch := make(chan []byte)
            client = &UDPPacketClient{"host", send_ch, recv_ch, nil, addr}
            go client.doForwardSend(u.send_ch)

            clientMapLock.Lock()
            clientMap[addr.String()] = client
            clientMapLock.Unlock()

            u.accept_ch <- client
        }

        client.recv_ch <- pkt[0:n]
    }
}

func (u *UDPTransport) sendPackets() {
    var msg udpMessage
    var err error

    // TODO: some way to stop this
    for {
        msg = <-u.send_ch

        log.Printf("Writing packet of length %d...\n", len(msg.msg))
        _, err = u.listen.WriteToUDP(msg.msg, msg.addr)
        if err != nil {
            log.Printf("Error writing packet: %s\n", err)
            break
        }
    }
}
