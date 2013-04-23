package main

import (
    "bytes"
    "encoding/binary"
    "flag"
    "fmt"
    "io"
    "log"
    "math/rand"
    "crypto/hmac"
    "crypto/sha256"
    "net"
    "os"
    "strings"
    "time"
    "encoding/hex"
)

const MAJOR_VER = 1
const MINOR_VER = 0

// Message that the client sends to the server to see if the server is
// responding, and, if so, start the authentication procedure.
type ClientInitialRequest struct {
    // Client's hostname
    hostname string
}

// Message the server sends to the client to check version and start the
// authentication process.
type ServerInitialResponse struct {
    // Server version
    majorVer int
    minorVer int

    // Challenge for the client
    challenge string
}

// Message the client sends to the server to complete authentication.
type ClientChallengeResponse struct {
    // Response from the challenge
    challengeResp string
}

// Authentication result from the server.
type ServerAuthenticationResult struct {
    // Success or failure.
    authenticationSuccess bool
}

/* The protocol for communication is simple:
 *  - In the negotiation phase, the client sends messages back and forth
 *    to the server to verify connectivity, check versions, and authenticate.
 *          Client                 Server
 *      check_version   -->          *
 *            *         <--     server_version
 *                                   +
 *                               challenge
 *         response     -->          *
 *            *         <--        result
 *
 *    The messages can be distinguished by the first byte, as follows:
 *          0x00    data
 *          0x01    check_version
 *          0x02    server_challenge
 *          0x03    client_challenge_response
 *          0x04    server_auth_result
 *
 *    Note: if the challenge response is not received within 10 seconds, then
 *    the server will close the connection without sending a result.
 *
 *  - After authentication succeeds, all further messages are just binary blobs
 *    that contain packets to be forwarded.  Note that the underlying transport
 *    may impose some overhead (e.g. the TCP transport will prefix packets with
 *    the length, since TCP is a stream-oriented protocol, and UDP needs to
 *    include a header for reliable delivery).
 */

// This interface represents a single connected client.
type PacketClient interface {
    // Send a single packet, error if necessary.  Will block for some time if
    // no packets can currently be sent - e.g. if the send window is full on
    // some transports.
    SendPacket(pkt []byte) error

    // Get a single packet, returning it and an error.  Will block for some
    // time if no packet is available, and will then return an error if it
    // times out.
    GetPacket() ([]byte, error)

    // Close this transport down.
    Close()

    // This can return whatever - mostly used for helpful debugging.
    Describe() string
}

// This interface represents a server - something that will accept clients.
type GenericServer interface {
    // Accept a single client and return it.
    AcceptClient() (PacketClient, error)
}

// Global options
var device = flag.String("d", "", "the tun/tap device to connect to")
var password = flag.String("pass", "insecure", "password for authentication")

// Client options
var is_client = flag.Bool("c", false, "operate in client mode")
var method = flag.String("m", "all", "methods to try, as comma-seperated list (tcp/udp/icmp/dns/all)")

// Server options
var is_server = flag.Bool("s", false, "operate in server mode")

// ============================================================================
// =================================== MAIN ===================================
// ============================================================================

func usage() {
    fmt.Fprintf(os.Stderr, "Usage: %s [options] server\n", os.Args[0])
    flag.PrintDefaults()
    os.Exit(2)
}

func main() {
    // Start by printing a header and parsing flags.
    flag.Usage = usage
    flag.Parse()

    // Seed PRNG.
    rand.Seed(time.Now().UTC().UnixNano())

    // Check client / server.
    if !*is_client && !*is_server {
        fmt.Fprintf(os.Stderr, "Did not specify client or server!\n")
        os.Exit(1)
    } else if *is_client && *is_server {
        fmt.Fprintf(os.Stderr, "Cannot specify both client and server mode!\n")
        os.Exit(1)
    }

    // Verify that we have a device and open it.
    if *device == "" {
        fmt.Fprintf(os.Stderr, "No TUN/TAP device given!\n")
        os.Exit(1)
    }

    // TODO: add in.  this requires root, so not for testing.
    /* tuntap, err := os.OpenFile(*device, os.O_RDWR, 0666) */
    /* if err != nil { */
    /*    log.Fatal(err) */
    /* } */
    /* defer tuntap.Close() */

    // Kickoff the client or server.
    if *is_client {
        run_client()
    } else {
        run_server()
    }

    // TODO: run configuration (route adding, iptables NATing, etc.)
}

// ============================================================================
// ================================== SERVER ==================================
// ============================================================================

func run_server() {
    // Kick off each of our individual transports.  Each transport runs its own
    // goroutine.  When a transport receives a connection from a new client,
    // it will kick off a new goroutine with the logic to authenticate and
    // then handle the connection.

    log.Println("Starting TCP server...")
    tcpServer, err := NewTCPPacketServer("")
    if err != nil {
        log.Printf("Error creating TCP server: %s\n", err)
    } else {
        startPacketServer(tcpServer, "TCP")
        log.Println("Successfully started TCP server")
    }

    ch := make(chan bool)
    <-ch
}

func startPacketServer(server GenericServer, method string) {
    go (func() {
        for {
            client, _ := server.AcceptClient()
            log.Printf("Accepted new client on %s transport\n", method)

            go handle_new_client(&client)
        }
    })()
}

const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func randomBytes(l int) []byte {
    bytes := make([]byte, l)
    for i := 0; i < l; i++ {
        bytes[i] = charset[rand.Intn(len(charset))]
    }

    return []byte{'a', 'b', 'c', 'd'}
    return bytes
}

// Authenticate and then handle the client.
func handle_new_client(client *PacketClient) {
    // At the end of this function, we're to close the client.
    defer (*client).Close()

    // First off, send the client a challenge.  Generate a random nonce...
    nonce := randomBytes(32)

    // Send the challenge.
    (*client).SendPacket(nonce)

    // Wait for one of three things:
    //  - Successful authentication
    //  - Unsuccessful authentication
    //  - Timeout
    res := make(chan []byte)
    go (func() {
        for {
            pkt, err := (*client).GetPacket()
            if err == nil {
                res <- pkt
                return
            }
        }
    })()

    // Calculate the proper response to the challenge.
    hm := hmac.New(sha256.New, []byte(*password))
    _, err := hm.Write(nonce)
    if err != nil {
        log.Printf("Error computing HMAC: %s\n", err)
        return
    }
    expected := hex.EncodeToString(hm.Sum(nil))

    select {
    case resp := <-res:
        // If authentication failed...
        if string(resp) != expected {
            log.Printf("Authentication failure")
            (*client).SendPacket([]byte("failure"))
            return
        } else {
            log.Println("Authentication success!")
        }
    case <-time.After(10 * time.Second):
        // Timeout!
        log.Printf("Authentication timeout")
        return
    }

    // Send a literal "success".
    (*client).SendPacket([]byte("success"))

    // We've got a valid, authenticated client here.  Read packets from the
    // transport and dump them to our TUN/TAP interface, and read packets from
    // the TUN/TAP interface and dump them to the client.
    for {
        select {}
    }
}

// ============================================================================
// ================================== CLIENT ==================================
// ============================================================================

func run_client() {
    // Verify we have a server address.
    args := flag.Args()
    if len(args) < 1 {
        fmt.Fprintf(os.Stderr, "No server address given!\n")
        os.Exit(1)
    }
    log.Printf("Holepunching with server %s...\n", args[0])

    // Determine the method.
    var methods = strings.Split(*method, ",")
    if len(methods) == 1 && methods[0] == "all" {
        methods = []string{"tcp", "udp", "icmp", "dns"}
    }

    // Try each method.
    var client *PacketClient = nil
    for i := range methods {
        var curr PacketClient = nil
        var err error

        switch methods[i] {
        case "tcp":
            log.Printf("Trying TCP connection...")
            curr, err = NewTCPPacketClient(args[0])
        case "udp":
            log.Printf("Trying UDP connection...")
        case "icmp":
            log.Printf("Trying ICMP connection...")
        case "dns":
            log.Printf("Trying DNS connection...")
        }

        if curr != nil {
            log.Println("Successfully created transport, starting authentication...")
        } else {
            log.Printf("Error: no transport returned (error: %s)\n", err)
            continue
        }

        // Read a single packet.
        challenge, err := curr.GetPacket()
        if err != nil {
            log.Printf("Error while reading authentication packet: %s\n", err)
            continue
        }

        // Compute the HMAC of the challenge.
        hm := hmac.New(sha256.New, []byte(*password))
        _, err = hm.Write(challenge)
        if err != nil {
            log.Printf("Error computing HMAC: %s\n", err)
            return
        }
        expected := hex.EncodeToString(hm.Sum(nil))

        // Send to the server.
        curr.SendPacket([]byte(expected))

        // Read a packet - should contain a literal "success".
        resp, err := curr.GetPacket()
        if err != nil {
            log.Printf("Error after authentication: %s", err)
            continue
        }
        if bytes.Equal(resp, []byte("success")) {
            log.Println("Authentication success!")
            client = &curr
            break
        } else if bytes.Equal(resp, []byte("failure")) {
            log.Println("Error with authentication - invalid password")
            continue
        } else {
            log.Println("Error with authentication - got unexpected response")
            continue
        }
    }

    if client == nil {
        log.Fatal("Could not start any transports!")
    }

    log.Printf("Connected with transport: %s\n", (*client).Describe())
}

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
    incoming chan *TCPPacketClient
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
    ch := make(chan *TCPPacketClient)
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

func (t *TCPPacketServer) AcceptClient() (PacketClient, error) {
    client := <-t.incoming
    return client, nil
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
        log.Printf("Reading %d bytes...\n", length)
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

func (t *TCPPacketClient) GetPacket() ([]byte, error) {
    // Read a packet, or timeout.
    select {
    case pkt := <-t.incoming:
        // Got it.
        return pkt, nil
    case <-time.After(5 * time.Second):
        // Fall through
    }

    // TODO: return a timeout error
    return nil, nil
}

func (t *TCPPacketClient) Describe() string {
    return "TCPPacketClient"
}

func (t *TCPPacketClient) Close() {
    // Do nothing.
}
