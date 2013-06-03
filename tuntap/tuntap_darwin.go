// +build darwin
package tuntap

import (
    "io"
    "log"
    "os"
    "syscall"
    "time"
)

type DarwinTunTap struct {
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

    tuntapDev, err := os.OpenFile("/dev/"+name, os.O_RDWR, 0666)
    if err != nil {
        log.Printf("Error opening file: %s\n", err)
        return nil, err
    }

    // Create channels.
    packets := make(chan []byte)
    eof := make(chan bool)

    // Create structure.
    tuntap := DarwinTunTap{tuntapDev, name, packets, eof}

    // Return our device
    return &tuntap, nil
}

func (t *DarwinTunTap) Start() {
    // Create goroutine that reads packets.
    go packetReader(t)
}

func FD_SET(fd int, p *syscall.FdSet) {
    // From the header files:
    //
    // #define  __DARWIN_NBBY       8               /* bits in a byte */
    // #define __DARWIN_NFDBITS (sizeof(__int32_t) * __DARWIN_NBBY) /* bits per mask */
    //
    // #define  __DARWIN_FD_SET(n, p)   do {
    //      int __fd = (n);
    //      ((p)->fds_bits[__fd/__DARWIN_NFDBITS] |= (1<<(__fd % __DARWIN_NFDBITS)));
    // } while(0)
    n, k := fd/32, fd%32
    p.Bits[n] |= (1 << uint32(k))
}

func FD_CLEAR(fd int, p *syscall.FdSet) {
    // Read above for information.
    n, k := fd/32, fd%32
    p.Bits[n] &= ^(1 << uint32(k))
}

func packetReader(t *DarwinTunTap) {
    packet := make([]byte, 65535)
    fds := new(syscall.FdSet)
    fd := int(t.file.Fd())
    FD_SET(fd, fds)

    for {
        // On Mac OS X, reading from the tun/tap device will do strange
        // things.  We need to use syscall.Select.
        syscall.Select(fd+1, fds, nil, nil, nil)

        // Actually read.
        n, err := t.file.Read(packet)
        if err == io.EOF {
            t.eof <- true
            return
        } else if err != nil {
            log.Printf("Error reading from tuntap: %s\n", err)

            // This wait is to stop us from getting stuck in an infinite loop
            // of reads that all error, and consuming 100% CPU forever.
            <-time.After(100 * time.Millisecond)
            continue
        }

        t.packets <- packet[0:n]
    }
}

func (t *DarwinTunTap) RecvChannel() chan []byte {
    return t.packets
}

func (t *DarwinTunTap) EOFChannel() chan bool {
    return t.eof
}

func (t *DarwinTunTap) Write(pkt []byte) error {
    _, err := t.file.Write(pkt)
    return err
}

func (t *DarwinTunTap) Name() string {
    return t.name
}

func (t *DarwinTunTap) Close() {
    t.file.Close()
}
