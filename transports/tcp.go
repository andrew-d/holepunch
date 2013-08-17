package transports

import (
    "encoding/binary"
    "fmt"
    "log"
    "net"
)

type TCPPacketClient struct {
    host    string
    send_ch chan []byte
    recv_ch chan []byte
    conn    net.Conn
}

const TCP_PORT = 44461

func NewTCPPacketClient(server string) (*TCPPacketClient, error) {
    host := fmt.Sprintf("%s:%d", server, TCP_PORT)

    conn, err := net.Dial("tcp", host)
    if err != nil {
        log.Printf("Error connecting with TCP: %s", err)
        return nil, err
    }

    return newTcpClientFromConn(host, conn), nil
}

func newTcpClientFromConn(host string, conn net.Conn) *TCPPacketClient {
    send_ch := make(chan []byte)
    recv_ch := make(chan []byte)

    ret := &TCPPacketClient{host, send_ch, recv_ch, conn}

    go ret.doSend()
    go ret.doRecv()

    return ret
}

func (c *TCPPacketClient) doSend() {
    var pkt []byte
    var err error
    var length = make([]byte, 2)

    for {
        // TODO: select on "stop" channel
        pkt = <-c.send_ch

        binary.LittleEndian.PutUint16(length, uint16(len(pkt)))
        _, err = c.conn.Write(length)
        if err != nil {
            log.Printf("Error writing length: %s\n", err)
            break
        }

        _, err = c.conn.Write(pkt)
        if err != nil {
            log.Printf("Error writing packet: %s\n", err)
            break
        }
    }
}

func (c *TCPPacketClient) doRecv() {
    var err error
    var ilen uint16
    var pkt []byte
    var length = make([]byte, 2)

    for {
        _, err = c.conn.Read(length)
        if err != nil {
            log.Printf("Error reading length: %s\n", err)
            break
        }
        ilen = binary.LittleEndian.Uint16(length)

        pkt = make([]byte, ilen)

        _, err = c.conn.Read(pkt)
        if err != nil {
            log.Printf("Error reading packet: %s\n", err)
            break
        }

        // TODO: select on "stop" channel
        c.recv_ch <- pkt
    }
}

func (c *TCPPacketClient) SendChannel() chan []byte {
    return c.send_ch
}

func (c *TCPPacketClient) RecvChannel() chan []byte {
    return c.recv_ch
}

func (c *TCPPacketClient) Close() {
    c.conn.Close()
}

func (c *TCPPacketClient) IsReliable() bool {
    return true
}

func (c *TCPPacketClient) Describe() string {
    return "TCPPacketClient"
}

type TCPTransport struct {
    listen    net.Listener
    accept_ch chan PacketClient
}

func NewTCPTransport(bindTo string) (*TCPTransport, error) {
    host := fmt.Sprintf("%s:%d", bindTo, TCP_PORT)

    listener, err := net.Listen("tcp", host)
    if err != nil {
        return nil, err
    }

    client_ch := make(chan PacketClient)
    trans := &TCPTransport{listener, client_ch}

    go trans.acceptConnections()

    return trans, nil
}

func (t *TCPTransport) acceptConnections() {
    log.Println("Started accepting clients")

    for {
        conn, err := t.listen.Accept()
        if err != nil {
            log.Printf("Error accepting client: %s\n", err)
            continue
        }

        client := newTcpClientFromConn("host", conn)
        t.accept_ch <- client
    }
}

func (t *TCPTransport) AcceptChannel() chan PacketClient {
    return t.accept_ch
}
