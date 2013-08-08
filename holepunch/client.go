package holepunch

import (
    flag "github.com/ogier/pflag"
    "fmt"
    "log"
    "os"
    "strings"

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

    var holepunch_server string
    cmd_args := flags.Args()
    if len(cmd_args) < 1 {
        fmt.Fprintf(os.Stderr, "No server address given!\n\n")
        fmt.Fprintf(os.Stderr, "Usage:\n")
        fmt.Fprintf(os.Stderr, "  holepunch client [options] server_addr\n\n")
        os.Exit(1)
    } else {
        holepunch_server = cmd_args[0]
    }

    tuntap := getTuntap(true)
    defer tuntap.Close()

    // Use a different goroutine, so the main routine can wait for signals.
    go startClient(tuntap, holepunch_server)
}

func StopClient() {
    // TODO: fill me in!
}

func startClient(tuntap tuntap.Device, hpserver string) {
    log.Printf("Holepunching with server %s...\n", hpserver)

    // Determine the method.
    var methods = strings.Split(method, ",")
    if len(methods) == 1 && methods[0] == "all" {
        methods = []string{"tcp", "udp", "icmp", "dns"}
    }
}
