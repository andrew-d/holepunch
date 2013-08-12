package holepunch

import (
    "bytes"
    "crypto/hmac"
    "crypto/sha256"
    "encoding/hex"
    "fmt"
    flag "github.com/ogier/pflag"
    "log"
    "os"
    "strings"
    "time"

    "github.com/andrew-d/holepunch/transports"
    "github.com/andrew-d/holepunch/tuntap"
)

// Client options
var method string
var server_addr string

func RunClient(args []string) {
    flags := flag.NewFlagSet("client", flag.ExitOnError)
    addCommonOptions(flags)

    flags.StringVar(&method, "m", "all", "methods to try, as comma-seperated list (tcp/udp/icmp/dns/all)")
    flags.StringVar(&server_addr, "server", "10.93.0.1", "ip address of the server")

    flags.Parse(args)

    cmd_args := flags.Args()
    if len(cmd_args) < 1 {
        fmt.Fprintf(os.Stderr, "No server address given!\n\n")
        fmt.Fprintf(os.Stderr, "Usage:\n")
        fmt.Fprintf(os.Stderr, "  holepunch client [options] server_addr\n\n")
        os.Exit(1)
    } else {
        // Use a different goroutine, so the main routine can wait for signals.
        tt := getTuntap(true)
        go startClient(tt, cmd_args[0])
    }
}

func StopClient() {
    // TODO: fill me in!
}

func startClient(tt tuntap.Device, hpserver string) {
    defer tt.Close()
    log.Printf("Holepunching with server %s...\n", hpserver)

    methods := strings.Split(method, ",")
    if len(methods) == 1 && methods[0] == "all" {
        methods = []string{"tcp", "udp", "icmp", "dns"}
    }

    var conn transports.PacketClient
    var err error

    for _, m := range methods {
        var curr_conn transports.PacketClient

        err = nil
        switch m {
        case "tcp":
            curr_conn, err = transports.NewTCPPacketClient(hpserver)

        default:
            log.Printf("Unknown method: %s\n", m)
            continue
        }

        if err != nil {
            log.Printf("Error creating transport '%s': %s\n", m, err)
            continue
        }

        if doAuth(curr_conn) {
            conn = curr_conn
            break
        }
    }

    if conn == nil {
        log.Printf("Could not create connection to server, exiting...\n")
        return
    }
    log.Printf("Connected to server (reliable = %t)\n", conn.IsReliable())

    // Set up encryption.
    enc_conn, err := transports.NewEncryptedPacketClient(conn, "foobar")
    if err != nil {
        log.Printf("Could not initialize encryption: %s\n", err)
        conn.Close()
        return
    }
    recv_ch := enc_conn.RecvChannel()
    send_ch := enc_conn.SendChannel()

    defer enc_conn.Close()

    for {
        // TODO: some way of stopping this
        select {
        case from_server := <-recv_ch:
            log.Printf("server --> tuntap (%d bytes)\n", len(from_server))
            tt.Write(from_server)

        case from_tuntap := <-tt.RecvChannel():
            log.Printf("tuntap --> server (%d bytes)\n", len(from_tuntap))
            send_ch <- from_tuntap

        case <-tt.EOFChannel():
            log.Println("EOF received from TUN/TAP device, exiting...")
            return
        }
    }
}

func doAuth(conn transports.PacketClient) bool {
    // Authentication times out after 10 seconds.
    timeout_ch := time.After(10 * time.Second)
    send_ch := conn.SendChannel()
    recv_ch := conn.RecvChannel()

    // Receive the nonce from the server.
    var nonce []byte
    select {
    case nonce = <-recv_ch:
        // fall through
    case <-timeout_ch:
        log.Printf("Client authentication timed out: receiving nonce\n")
        return false
    }

    hm := hmac.New(sha256.New, []byte(password))
    _, err := hm.Write(nonce)
    if err != nil {
        log.Printf("Error computing HMAC: %s\n", err)
        return false
    }

    resp := make([]byte, 64)
    hex.Encode(resp, hm.Sum(nil))

    select {
    case send_ch <- resp:
        // fall through
    case <-timeout_ch:
        log.Printf("Client authentication timed out: sending respond\n")
        return false
    }

    // Wait for a response from the server.
    var serv_resp []byte
    select {
    case serv_resp = <-recv_ch:
        // fall through
    case <-timeout_ch:
        log.Printf("Client authentication timed out: waiting for confirmation\n")
        return false
    }

    // Check response.
    if bytes.Equal(serv_resp, []byte("success")) {
        log.Printf("Authentication success\n")
        return true
    } else {
        log.Printf("Authentication failure: %s\n", serv_resp)
        return false
    }
}
