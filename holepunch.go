package main

import (
    "bytes"
    "encoding/binary"
    "flag"
    "fmt"
    "log"
    "net"
    "os"
    "io"
    "strings"
    "time"
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

// Global options
var device = flag.String("d", "", "the tun/tap device to connect to")
var password = flag.String("pass", "", "password for authentication")

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
}

// Authenticate and then handle the client.
func handle_new_client() {
    // First off, send the client a challenge.
    //client.Send("challenge")

    // Wait for one of three things:
    //  - Successful authentication
    //  - Unsuccessful authentication
    //  - Timeout
    res := make(chan []byte)
    select {
    case resp := <-res:
        // Validate auth.
        _ = resp

        // If authentication failed...
        if false {
            log.Printf("Authentication failure")
            return
        }
    case <-time.After(10 * time.Second):
        // Timeout!
        log.Printf("Authentication timeout")
        return
    }

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
            curr, err = NewTCPPackettClient(args[0])
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

const TCP_PORT = 44461

// Initializer.
func NewTCPPackettClient(server string) (*TCPPacketClient, error) {
    host := fmt.Sprintf("%s:%d", server, TCP_PORT)

    // Connect to this client.
    conn, err := net.Dial("tcp", host)
    if err != nil {
        log.Printf("Error connecting with TCP: %s", err)
        return nil, err
    }

    // Make packet channel.
    incoming := make(chan []byte)

    // This goroutine handles reads that can time out.
    go (func() {
        for {
            var length int
            lenb := make([]byte, binary.Size(length))

            // Try to read the length.  On an EOF error, we exit this
            // goroutine, and on all other errors just keep reading lengths.
            _, err := conn.Read(lenb)
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
            _, err = conn.Read(pkt)
            if err == io.EOF {
                log.Println("EOF received on packet read")
                break
            } else if err != nil {
                log.Printf("Error while reading packet: %s\n", err)
                continue
            }

            // Got a packet - send to our channel.  Note that we will then
            // loop around to the top again, and continue reading packets.
            incoming <- pkt
        }
    })()

    // All connected!  Return the transport.
    ret := &TCPPacketClient{host, &conn, incoming}
    return ret, nil
}

func (t *TCPPacketClient) SendPacket(pkt []byte) error {
    // Encode length of packet.
    buf := &bytes.Buffer{}
    err := binary.Write(buf, binary.LittleEndian, len(pkt))
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
