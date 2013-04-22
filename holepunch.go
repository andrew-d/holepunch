package main

import (
    "fmt"
    "log"
    "os"
    // "net"
    "flag"
    "strings"

    "github.com/andrew-d/holepunch/transports"
)

type PacketTransport interface {
    SendPacket(pkt []byte) error
    GetPacket() ([]byte, error)
    Close()
    Describe() string
}

var device = flag.String("d", "", "the tun/tap device to connect to")
var method = flag.String("m", "all", "methods to try, as comma-seperated list (tcp/udp/icmp/dns/all)")

func usage() {
    fmt.Fprintf(os.Stderr, "Usage: %s [options] server\n", os.Args[0])
    flag.PrintDefaults()
    os.Exit(2)
}

func main() {
    flag.Usage = usage
    flag.Parse()

    // Verify we have a server address.
    args := flag.Args()
    if len(args) < 1 {
        fmt.Println("No server address given!")
        os.Exit(1)
    }

    // Verify that we have a device and open it.
    /* if *device == "" {
           fmt.Println("No TUN/TAP device given!")
           os.Exit(1)
       }

       tuntap, err := os.OpenFile(*device, os.O_RDWR, 0666)
       if err != nil {
           log.Fatal(err)
       }
       defer tuntap.Close() */

    log.Printf("Holepunching with server %s...\n", args[0])

    // Determine the method.
    var methods = strings.Split(*method, ",")
    if len(methods) == 1 && methods[0] == "all" {
        methods = []string{"tcp", "udp", "icmp", "dns"}
    }

    // Try each method.
    var t PacketTransport = nil
    for i := range methods {
        var currTransport PacketTransport = nil

        switch methods[i] {
        case "tcp":
            log.Printf("Trying TCP connection...")
            currTransport = nil
        case "udp":
            log.Printf("Trying UDP connection...")
            currTransport = TryUDP(args[0])
        case "icmp":
            log.Printf("Trying ICMP connection...")
            currTransport = nil
        case "dns":
            log.Printf("Trying DNS connection...")
            currTransport = nil
        }

        // Test this method.
        if currTransport == nil {
            log.Println("Error: no transport returned")
        } else if TestTransport(currTransport) {
            t = currTransport
        } else {
            log.Printf("Error: transport %s did not test successfully\n", currTransport.Describe())
            currTransport.Close()
        }
    }

    if t == nil {
        log.Fatal("Could not start any transports!")
    }

    log.Printf("Connected with transport: %s\n", t.Describe())
}

func TestTransport(t PacketTransport) bool {
    return false
}

func TryUDP(server string) *transports.UDPTransport {
    u, err := transports.NewUDPTransport(server)
    if err != nil {
        log.Printf("Error starting UDP transport: %s\n", err)
        return nil
    }

    log.Println("Successfully started UDP transport")
    return u
}
