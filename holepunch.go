package main

import (
    "fmt"
    "log"
    "math/rand"
    "os"
    "os/signal"
    "time"

    "github.com/andrew-d/holepunch/holepunch"
)

const (
    UNKNOWN = 0
    SERVER  = 1
    CLIENT  = 2
)

var which = UNKNOWN

func main() {
    // Setup logging.
    log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

    // Seed PRNG.
    rand.Seed(time.Now().UTC().UnixNano())

    // Deal with signals.
    // TODO: do we really need another goroutine for this?
    sig_ch := make(chan os.Signal, 1)
    done_ch := make(chan bool)
    go handleSignals(sig_ch, done_ch)
    signal.Notify(sig_ch, os.Interrupt, os.Kill)

    // Check subcommand.
    if len(os.Args) < 2 {
        fmt.Println("Usage:")
        fmt.Println("  holepunch (server|client) [options]")
        fmt.Println("")
        os.Exit(1)
    }
    cmd := os.Args[1]

    switch cmd {
    case "client":
        holepunch.RunClient(os.Args[2:])

    case "server":
        holepunch.RunServer(os.Args[2:])

    default:
        fmt.Fprintf(os.Stderr, "Usage:\n")
        fmt.Fprintf(os.Stderr, "  holepunch (server|client) [options]\n\n")
        os.Exit(1)
    }

    <-done_ch
}

func handleSignals(ch chan os.Signal, done chan bool) {
    // Tell the server / client to stop.
    switch which {
    case UNKNOWN:
        // Do nothing, just exit.

    case SERVER:
        holepunch.StopServer()

    case CLIENT:
        holepunch.StopClient()
    }

    done <- true
}
