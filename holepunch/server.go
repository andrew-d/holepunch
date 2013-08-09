package holepunch

import (
    "crypto/hmac"
    "crypto/subtle"
    "crypto/sha256"
    "encoding/hex"
    flag "github.com/ogier/pflag"
    //"fmt"
    "log"
    "time"

    "github.com/andrew-d/holepunch/transports"
    "github.com/andrew-d/holepunch/tuntap"
)

func RunServer(args []string) {
    flags := flag.NewFlagSet("server", flag.ExitOnError)
    addCommonOptions(flags)
    flags.Parse(args)

    // We start the transports in another goroutine, so our main routine can
    // return (and wait for signals).
    // Note: The startTransports function takes ownership (and closes) the
    // tuntap device.
    tt := getTuntap(false)
    go startTransports(tt)
}

func StopServer() {
    // TODO: fill me in!
}

func startTransports(tt tuntap.Device) {
    defer tt.Close()

    trans, err := transports.NewTCPTransport("0.0.0.0")
    if err != nil {
        log.Printf("Error starting TCP transport: %s\n", err)
        return
    }

    // Repeatedly accept clients.
    ch := trans.AcceptChannel()
    for {
        // TODO: have some way of stopping this
        client := <-ch

        go handleNewClient(tt, client)
    }
}

// Authenticate and then handle the client.
func handleNewClient(tt tuntap.Device, client transports.PacketClient) {
    log.Println("Accepted new client")
    defer client.Close()

    send_ch := client.SendChannel()
    recv_ch := client.RecvChannel()

    nonce := randomBytes(32)
    send_ch <- nonce

    // Wait for one of three things:
    //  - Successful authentication
    //  - Unsuccessful authentication
    //  - Timeout
    hm := hmac.New(sha256.New, []byte(password))
    _, err := hm.Write(nonce)
    if err != nil {
        log.Printf("Error computing HMAC: %s\n", err)
        return
    }

    expected := make([]byte, 64)
    hex.Encode(expected, hm.Sum(nil))

    select {
    case resp := <-recv_ch:
        // Note that it is IMPORTANT we use this function here, to avoid leaking
        // timing information.  Then, if authentication fails, we just outright
        // exit, and let the deferred close handle things.
        if subtle.ConstantTimeCompare(resp, expected) != 1 {
            log.Printf("Authentication failure")
            send_ch <- []byte("failure")
            return
        } else {
            log.Println("Authentication success!")
        }
    case <-time.After(10 * time.Second):
        // Timeout!
        log.Printf("Authentication timeout")
        return
    }

    send_ch <- []byte("success")

    for {
        select {
        case from_client := <-recv_ch:
            log.Printf("client --> tuntap (%d bytes)\n", len(from_client))
            tt.Write(from_client)

        case from_tuntap := <-tt.RecvChannel():
            log.Printf("tuntap --> client (%d bytes)\n", len(from_tuntap))
            send_ch <- from_tuntap

        case <-tt.EOFChannel():
            log.Println("EOF received from TUN/TAP device, exiting...")
            return
        }
    }
}
