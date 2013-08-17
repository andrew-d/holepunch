package holepunch

import (
    flag "github.com/ogier/pflag"
    //"fmt"
    "log"

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

    tcpt, err := transports.NewTCPTransport("0.0.0.0")
    if err != nil {
        log.Printf("Error starting TCP transport: %s\n", err)
        return
    }

    udpt, err := transports.NewUDPTransport("0.0.0.0")
    if err != nil {
        log.Printf("Error starting UDP transport: %s\n", err)
        return
    }

    // Repeatedly accept clients.
    tcp_ch := tcpt.AcceptChannel()
    udp_ch := udpt.AcceptChannel()

    var client transports.PacketClient
    for {
        // TODO: have some way of stopping this
        select {
        case client = <-tcp_ch:
        case client = <-udp_ch:
        }

        go handleNewClient(tt, client)
    }
}

// Authenticate and then handle the client.
func handleNewClient(tt tuntap.Device, client transports.PacketClient) {
    log.Printf("Accepted new client (reliable = %t)\n", client.IsReliable())

    // Set up encryption.
    enc_client, err := transports.NewEncryptedPacketClient(client, "foobar")
    if err != nil {
        log.Printf("Could not initialize encryption: %s\n", err)
        client.Close()
        return
    }
    defer enc_client.Close()

    recv_ch := enc_client.RecvChannel()
    send_ch := enc_client.SendChannel()

    for {
        select {
        case from_client := <-recv_ch:
            log.Printf("client --> tuntap (%d bytes)\n", len(from_client))
            err = tt.Write(from_client)
            if err != nil {
                log.Printf("Error writing: %s\n", err)
            }

        case from_tuntap := <-tt.RecvChannel():
            log.Printf("tuntap --> client (%d bytes)\n", len(from_tuntap))
            send_ch <- from_tuntap

        case <-tt.EOFChannel():
            log.Println("EOF received from TUN/TAP device, exiting...")
            return
        }
    }
}
