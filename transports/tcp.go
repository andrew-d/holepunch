package transports

import (
    "bytes"
    "encoding/binary"
    "fmt"
    "io"
    "log"
    "net"
)

// ============================================================================
// ============================== TCP TRANSPORT ===============================
// ============================================================================

type TCPPacketClient struct {
    host     string
    conn     *net.Conn
    incoming chan []byte
}

// TODO: single struct with flag for which?
type TCPPacketServer struct {
    host     string
    listener *net.Listener
    incoming chan PacketClient
}

const TCP_PORT = 44461

// Initializer.
func NewTCPPacketClient(server string) (*TCPPacketClient, error) {
    host := fmt.Sprintf("%s:%d", server, TCP_PORT)

    // Connect to this client.
    conn, err := net.Dial("tcp", host)
    if err != nil {
        log.Printf("Error connecting with TCP: %s", err)
        return nil, err
    }

    ret := startClientWithConn(&conn, host)
    return ret, nil
}

func startClientWithConn(conn *net.Conn, host string) *TCPPacketClient {
    // Make packet channel.
    incoming := make(chan []byte)

    // Make the client.
    ret := &TCPPacketClient{host, conn, incoming}

    // This goroutine handles reads that can time out.
    go startTcpClientRecv(ret)

    // Return client
    return ret
}

func NewTCPPacketServer(bindTo string) (*TCPPacketServer, error) {
    host := fmt.Sprintf("%s:%d", bindTo, TCP_PORT)

    listener, err := net.Listen("tcp", host)
    if err != nil {
        log.Printf("Error listening on %s: %s\n", host, err)
        return nil, err
    }

    // Create server.
    ch := make(chan PacketClient)
    server := &TCPPacketServer{bindTo, &listener, ch}

    // This goroutine will accept new clients.
    go (func() {
        for {
            conn, err := listener.Accept()
            if err != nil {
                log.Printf("Error accepting new client: %s\n", err)
                continue
            }

            // Create a new client.
            client := startClientWithConn(&conn, "foobar")

            // Send this client on our accepting channel.
            ch <- client
        }
    })()

    return server, nil
}

func (t *TCPPacketServer) AcceptChannel() chan PacketClient {
    return t.incoming
}

func startTcpClientRecv(tcpClient *TCPPacketClient) {
    for {
        var length int16
        lenb := make([]byte, 2)

        // Try to read the length.  On an EOF error, we exit this
        // goroutine, and on all other errors just keep reading lengths.
        _, err := (*tcpClient.conn).Read(lenb)
        if err == io.EOF {
            log.Println("EOF received on length read")
            break
        } else if err != nil {
            log.Printf("Error while reading length: %s\n", err)
            continue
        }

        // Decode the length.
        err = binary.Read(bytes.NewBuffer(lenb), binary.LittleEndian, &length)
        if err != nil {
            log.Printf("Error decoding length: %s\n", err)
            continue
        }

        // Read this many bytes.
        pkt := make([]byte, length)
        _, err = (*tcpClient.conn).Read(pkt)
        if err == io.EOF {
            log.Println("EOF received on packet read")
            break
        } else if err != nil {
            log.Printf("Error while reading packet: %s\n", err)
            continue
        }

        // Got a packet - send to our channel.  Note that we will then
        // loop around to the top again, and continue reading packets.
        tcpClient.incoming <- pkt
    }

    // TODO: we should have some way of signalling that we're done here - and
    // perhaps some other way of manually breaking out of the loop.  Perhaps a
    // 'chan bool' that we send to when we hit here?
}

func (t *TCPPacketClient) SendPacket(pkt []byte) error {
    // Encode length of packet.
    buf := &bytes.Buffer{}
    l := int16(len(pkt))
    err := binary.Write(buf, binary.LittleEndian, l)
    if err != nil {
        return err
    }

    // Send length of packet, then the packet itself.
    _, err = (*t.conn).Write(buf.Bytes())
    if err != nil {
        return err
    }

    _, err = (*t.conn).Write(pkt)
    if err != nil {
        return err
    }

    return nil
}

func (t *TCPPacketClient) PacketChannel() chan []byte {
    return t.incoming
}

func (t *TCPPacketClient) Describe() string {
    return "TCPPacketClient"
}

func (t *TCPPacketClient) Close() {
    // Do nothing.
}
