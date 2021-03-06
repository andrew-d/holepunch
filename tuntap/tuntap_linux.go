// +build linux

package tuntap

import (
    "io"
    "log"
    "os"
    "syscall"
    "unsafe"
)

type LinuxTunTap struct {
    file     *os.File
    name     string
    packets  chan []byte
    exit     chan bool
    finished chan bool
}

func GetTuntapDevice() (Device, error) {
    name, err := getTuntapName()
    if err != nil {
        log.Printf("Error getting name: %s\n", err)
        return nil, err
    }
    log.Printf("Using name: %s\n", name)

    file, err := os.OpenFile("/dev/net/tun", os.O_RDWR, 0)
    if err != nil {
        log.Printf("Error opening file: %s\n", err)
        return nil, err
    }
    log.Printf("TUN fd = %d\n", file.Fd())

    // Create the request to send.
    var req ifReq
    req.Flags = iffOneQueue | iffTun | iffNoPI
    copy(req.Name[:15], name)

    // Send the request.
    _, _, serr := syscall.Syscall(syscall.SYS_IOCTL,
        file.Fd(),
        uintptr(syscall.TUNSETIFF),
        uintptr(unsafe.Pointer(&req)))
    if serr != 0 {
        log.Printf("Error with syscall: %s\n", err)
        return nil, err
    }

    packets := make(chan []byte)
    finished := make(chan bool)
    exit := make(chan bool)

    tuntap := &LinuxTunTap{file, name, packets, finished, exit}
    return tuntap, nil
}

func (t *LinuxTunTap) Start() {
    go packetReader(t)
}

func packetReader(t *LinuxTunTap) {
    var n int
    var err error
    var exit bool
    packet := make([]byte, 65535)

    for {
        n, err = t.file.Read(packet)

        // Check if we are to exit.
        select {
        case exit = <-t.exit:
        default:
        }
        if exit {
            log.Printf("Exit signal received\n")
            break
        }

        if err == io.EOF || n == 0 {
            log.Printf("EOF reading from TUN: %s", err)
            break
        } else if err != nil {
            log.Printf("Error reading from tuntap: %s\n", err)
            continue
        }

        t.packets <- packet[0:n]
    }

    log.Printf("Done")
    t.finished <- true
}

func (t *LinuxTunTap) RecvChannel() chan []byte {
    return t.packets
}

func (t *LinuxTunTap) EOFChannel() chan bool {
    return t.finished
}

func (t *LinuxTunTap) Write(pkt []byte) error {
    _, err := t.file.Write(pkt)
    return err
}

func (t *LinuxTunTap) Name() string {
    return t.name
}

func (t *LinuxTunTap) Close() {
    t.file.Close()
    t.file = nil

    // See comments in tuntap_darwin for this function.
    t.exit <- true
    <-t.finished
}
