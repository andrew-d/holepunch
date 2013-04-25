package main

import (
    "bytes"
    "crypto/hmac"
    "crypto/sha256"
    "encoding/binary"
    "encoding/hex"
    "flag"
    "fmt"
    "io"
    "log"
    "math/rand"
    "net"
    "os"
    "os/exec"
    "strings"
    "time"

    "github.com/andrew-d/holepunch/tuntap"
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
    PacketChannel() chan []byte

    // Close this transport down.
    Close()

    // This can return whatever - mostly used for helpful debugging.
    Describe() string
}

// This interface represents a server - something that will accept clients.
type GenericServer interface {
    // Accept a single client and return it.
    AcceptChannel() chan PacketClient
}

// Global options
var ipaddr = flag.String("ip", "", "the IP address of the TUN/TAP device")
var netmask = flag.String("netmask", "255.255.0.0", "the netmask of the TUN/TAP device")
var password = flag.String("pass", "insecure", "password for authentication")

// Client options
var is_client = flag.Bool("c", false, "operate in client mode")
var method = flag.String("m", "all", "methods to try, as comma-seperated list (tcp/udp/icmp/dns/all)")
var server_addr = flag.String("server", "10.93.0.1", "ip address of the server")

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

    log.Println("Opening TUN/TAP device...")
    tuntap, err := tuntap.GetTuntapDevice()
    if err != nil {
        log.Fatal(err)
    }
    defer tuntap.Close()

    // Configure the device.
    log.Println("Configuring TUN/TAP device...")
    configureTuntap(*is_client, tuntap.Name())

    // Start reading from the TUN/TAP device.
    tuntap.Start()

    // Kickoff the client or server.
    if *is_client {
        runClient(tuntap)
    } else {
        runServer(tuntap)
    }

    // TODO: run configuration (route adding, iptables NATing, etc.)
}

func configureTuntap(is_client bool, devName string) {
    // Configure the TUN/TAP device.
    // Set default IP address, if needed.
    if len(*ipaddr) == 0 {
        if is_client {
            *ipaddr = "10.93.0.2"
        } else {
            *ipaddr = "10.93.0.1"
        }
    }

    // Need to run: ifconfig tunX 10.0.0.1 10.0.0.1 netmask 255.255.255.0 up
    var cmd *exec.Cmd
    if is_client {
        cmd = exec.Command("/sbin/ifconfig", devName, *ipaddr, *server_addr, "netmask", *netmask, "up")
    } else {
        cmd = exec.Command("/sbin/ifconfig", devName, *ipaddr, *ipaddr, "netmask", *netmask, "up")
    }

    out, err := cmd.Output()
    if err != nil {
        log.Printf("Error running configuration command: %s\n", err)
    } else {
        log.Printf("Configured successfully (output: '%s')\n", out)
    }

    <-time.After(1 * time.Second)
}

// ============================================================================
// ================================== SERVER ==================================
// ============================================================================

func runServer(tuntap tuntap.Device) {
    // Kick off each of our individual transports.  Each transport runs its own
    // goroutine.  When a transport receives a connection from a new client,
    // it will kick off a new goroutine with the logic to authenticate and
    // then handle the connection.

    log.Println("Starting TCP server...")
    tcpServer, err := NewTCPPacketServer("")
    if err != nil {
        log.Printf("Error creating TCP server: %s\n", err)
    } else {
        startPacketServer(tuntap, tcpServer, "TCP")
        log.Println("Successfully started TCP server")
    }

    ch := make(chan bool)
    <-ch
}

func startPacketServer(tuntap tuntap.Device, server GenericServer, method string) {
    go (func() {
        for {
            client := <-server.AcceptChannel()
            log.Printf("Accepted new client on %s transport\n", method)

            go handleNewClient(tuntap, &client)
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
func handleNewClient(tuntap tuntap.Device, client *PacketClient) {
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
        res <- (<-(*client).PacketChannel())
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
        select {
        case from_client := <-(*client).PacketChannel():
            // Got from client, so write to our tuntap.
            log.Printf("client --> tuntap (%d bytes)\n", len(from_client))
            tuntap.Write(from_client)
        case from_tuntap := <-tuntap.RecvChannel():
            // Got from client, so send to server.
            log.Printf("tuntap --> client (%d bytes)\n", len(from_tuntap))
            (*client).SendPacket(from_tuntap)
        case <-tuntap.EOFChannel():
            // Done!
            log.Println("EOF received from TUN/TAP device, exiting...")
            return
        }
    }
}

// ============================================================================
// ================================== CLIENT ==================================
// ============================================================================

func runClient(tuntap tuntap.Device) {
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
        var curr PacketClient
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

        if err != nil || curr == nil {
            log.Println("Error: no transport returned")
            continue
        }
        log.Println("Successfully created transport, starting authentication...")

        // Read a single packet, or timeout.
        var challenge []byte
        select {
        case challenge = <-curr.PacketChannel():
            // Fall through
        case <-time.After(5 * time.Second):
            challenge = nil
        }
        if challenge == nil {
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
        var resp []byte
        select {
        case resp = <-curr.PacketChannel():
            // Fall through
        case <-time.After(5 * time.Second):
            resp = nil
        }
        if resp == nil {
            log.Printf("Error after authentication: %s", err)
            continue
        } else if bytes.Equal(resp, []byte("success")) {
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

    // Read from both.
    for {
        select {
        case from_server := <-(*client).PacketChannel():
            // Got from server, so write to our tuntap.
            log.Printf("server --> tuntap (%d bytes)\n", len(from_server))
            tuntap.Write(from_server)
        case from_tuntap := <-tuntap.RecvChannel():
            // Got from client, so send to server.
            log.Printf("tuntap --> server (%d bytes)\n", len(from_tuntap))
            (*client).SendPacket(from_tuntap)
        case <-tuntap.EOFChannel():
            // Done!
            log.Println("EOF received from TUN/TAP device, exiting...")
            return
        }
    }
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

func (t *TCPPacketClient) PacketChannel() chan []byte {
    // Read a packet, or timeout.
    return t.incoming
}

func (t *TCPPacketClient) Describe() string {
    return "TCPPacketClient"
}

func (t *TCPPacketClient) Close() {
    // Do nothing.
}
