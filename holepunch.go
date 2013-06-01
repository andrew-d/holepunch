package main

import (
    "bytes"
    "crypto/hmac"
    "crypto/sha256"
    "encoding/hex"
    "flag"
    "fmt"
    "log"
    "math/rand"
    "os"
    "os/exec"
    "strings"
    "time"

    "github.com/andrew-d/holepunch/transports"
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

// Global options
var ipaddr string
var netmask string
var password string
var is_client bool

// Client options
var method string
var server_addr string

// Server options
// None!

// ============================================================================
// =================================== MAIN ===================================
// ============================================================================

func main() {
    // Check subcommand.
    if len(os.Args) < 2 {
        fmt.Println("Usage:")
        fmt.Println("  holepunch (server|client) [options]")
        fmt.Println("")
        os.Exit(1)
    }
    cmd := os.Args[1]
    flags := flag.NewFlagSet(cmd, flag.ExitOnError)

    flags.StringVar(&ipaddr, "ip", "", "the IP address of the TUN/TAP device")
    flags.StringVar(&netmask, "netmask", "255.255.0.0", "the netmask of the TUN/TAP device")
    flags.StringVar(&password, "pass", "insecure", "password for authentication")

    switch cmd {
    case "client":
        // Client.
        is_client = true
        flags.StringVar(&method, "m", "all", "methods to try, as comma-seperated list (tcp/udp/icmp/dns/all)")
        flags.StringVar(&server_addr, "server", "10.93.0.1", "ip address of the server")

    case "server":
        // Server
        is_client = false

    default:
        fmt.Fprintf(os.Stderr, "Usage:\n")
        fmt.Fprintf(os.Stderr, "  holepunch (server|client) [options]\n\n")
        os.Exit(1)
    }

    // Parse flags.
    flags.Parse(os.Args[2:])

    var holepunch_server string
    if is_client {
        // Verify we have a server address.
        args := flags.Args()
        if len(args) < 1 {
            fmt.Fprintf(os.Stderr, "No server address given!\n\n")
            fmt.Fprintf(os.Stderr, "Usage:\n")
            fmt.Fprintf(os.Stderr, "  holepunch client [options] server_addr\n\n")
            os.Exit(1)
        } else {
            holepunch_server = args[0]
        }
    }

    // Seed PRNG.
    rand.Seed(time.Now().UTC().UnixNano())

    log.Println("Opening TUN/TAP device...")
    tuntap, err := tuntap.GetTuntapDevice()
    if err != nil {
        log.Fatal(err)
    }
    defer tuntap.Close()

    // Configure the device.
    log.Println("Configuring TUN/TAP device...")
    configureTuntap(is_client, tuntap.Name())

    // Start reading from the TUN/TAP device.
    tuntap.Start()

    // Kickoff the client or server.
    if is_client {
        runClient(tuntap, holepunch_server)
    } else {
        runServer(tuntap)
    }

    // TODO: run configuration (route adding, iptables NATing, etc.)
}

func configureTuntap(is_client bool, devName string) {
    // Configure the TUN/TAP device.
    // Set default IP address, if needed.
    if len(ipaddr) == 0 {
        if is_client {
            ipaddr = "10.93.0.2"
        } else {
            ipaddr = "10.93.0.1"
        }
    }

    // Need to run: ifconfig tunX 10.0.0.1 10.0.0.1 netmask 255.255.255.0 up
    var cmd *exec.Cmd
    if is_client {
        cmd = exec.Command("/sbin/ifconfig", devName, ipaddr, server_addr, "netmask", netmask, "up")
    } else {
        cmd = exec.Command("/sbin/ifconfig", devName, ipaddr, ipaddr, "netmask", netmask, "up")
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
    tcpServer, err := transports.NewTCPPacketServer("")
    if err != nil {
        log.Printf("Error creating TCP server: %s\n", err)
    } else {
        startPacketServer(tuntap, tcpServer, "TCP")
        log.Println("Successfully started TCP server")
    }

    ch := make(chan bool)
    <-ch
}

func startPacketServer(tuntap tuntap.Device, server transports.GenericServer, method string) {
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
func handleNewClient(tuntap tuntap.Device, client *transports.PacketClient) {
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
    hm := hmac.New(sha256.New, []byte(password))
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

func runClient(tuntap tuntap.Device, hpserver string) {
    log.Printf("Holepunching with server %s...\n", hpserver)

    // Determine the method.
    var methods = strings.Split(method, ",")
    if len(methods) == 1 && methods[0] == "all" {
        methods = []string{"tcp", "udp", "icmp", "dns"}
    }

    // Try each method.
    var client *transports.PacketClient = nil
    for i := range methods {
        var curr transports.PacketClient
        var err error

        switch methods[i] {
        case "tcp":
            log.Printf("Trying TCP connection...")
            curr, err = transports.NewTCPPacketClient(hpserver)
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
        hm := hmac.New(sha256.New, []byte(password))
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
