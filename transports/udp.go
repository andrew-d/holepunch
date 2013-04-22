package transports

import (
    "fmt"
    "net"
)

// Simple UDP transport.
type UDPTransport struct {
    conn *net.UDPConn
    host string
}

func NewUDPTransport(server string) (*UDPTransport, error) {
    udpAddr, err := net.ResolveUDPAddr("udp", server + ":44460")
    if err != nil {
        return nil, err
    }

    conn, err := net.DialUDP("udp", nil, udpAddr)
    if err != nil {
        return nil, err
    }

    r := &UDPTransport{conn, server}

    return r, nil
}

func (t *UDPTransport) GetPacket() ([]byte, error) {
    var buff [65536]byte
    n, err := t.conn.Read(buff[0:])
    if err != nil {
        return nil, err
    }

    return buff[0:n], nil
}

func (t *UDPTransport) SendPacket(pkt []byte) error {
    _, err := t.conn.Write(pkt)
    return err
}

func (t* UDPTransport) Close() {
    t.conn.Close()
}

func (t* UDPTransport) Describe() string {
    return fmt.Sprintf("UDPTransport(host = %s)", t.host)
}
