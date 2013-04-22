package transports

// Simple UDP transport.
type UDPTransport struct {
    conn *net.UDPConn
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

    r := &UDPTransport{conn}

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
