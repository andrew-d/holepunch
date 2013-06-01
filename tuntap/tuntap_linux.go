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
    file    *os.File
    name    string
    packets chan []byte
    eof     chan bool
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

    // Create the request to send.
    var req ifReq
    req.Flags = iffOneQueue | iffTun
    copy(req.Name[:15], name)

    // Send the request.
    _, _, serr := syscall.Syscall(syscall.SYS_IOCTL, file.Fd(), uintptr(syscall.TUNSETIFF), uintptr(unsafe.Pointer(&req)))
    if serr != 0 {
        log.Printf("Error with syscall: %s\n", err)
        return nil, err
    }

    // Create channels.
    packets := make(chan []byte)
    eof := make(chan bool)

    // Create structure.
    tuntap := LinuxTunTap{file, name, packets, eof}

    return tuntap, nil
}

func (t LinuxTunTap) Start() {
    // Create goroutine that reads packets.
    go packetReader(&t)
}

func packetReader(t *LinuxTunTap) {
    var n int
    var err error
    packet := make([]byte, 65535)

    for {
        n, err = t.file.Read(packet)
        if err == io.EOF || n == 0 {
            log.Printf("%d / %s", n, err)
            t.eof <- true
            return
        } else if err != nil {
            log.Printf("Error reading from tuntap: %s\n", err)
            continue
        }

        t.packets <- packet[0:n]
    }
}

func (t LinuxTunTap) RecvChannel() chan []byte {
    return t.packets
}

func (t LinuxTunTap) EOFChannel() chan bool {
    return t.eof
}

func (t LinuxTunTap) Write(pkt []byte) error {
    _, err := t.file.Write(pkt)
    return err
}

func (t LinuxTunTap) Name() string {
    return t.name
}

func (t LinuxTunTap) Close() {
    t.file.Close()
}
