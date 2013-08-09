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
        which = CLIENT

    case "server":
        holepunch.RunServer(os.Args[2:])
        which = SERVER

    default:
        fmt.Fprintf(os.Stderr, "Usage:\n")
        fmt.Fprintf(os.Stderr, "  holepunch (server|client) [options]\n\n")
        os.Exit(1)
    }

    // Deal with signals.
    sig_ch := make(chan os.Signal, 1)
    signal.Notify(sig_ch, os.Interrupt, os.Kill)

    <-sig_ch

    switch which {
    case UNKNOWN:
        // Do nothing, just exit.

    case SERVER:
        holepunch.StopServer()

    case CLIENT:
        holepunch.StopClient()
    }

    // TODO: wait for server/client to finish before exiting

    log.Printf("Done\n")
}
