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

func fdSet(fd_set *syscall.FdSet, fd uintptr) {
    // From the header files:
    //
    // #define  __DARWIN_NBBY       8               /* bits in a byte */
    // #define __DARWIN_NFDBITS (sizeof(__int32_t) * __DARWIN_NBBY) /* bits per mask */
    //
    // #define  __DARWIN_FD_SET(n, p)   do {
    //      int __fd = (n);
    //      ((p)->fds_bits[__fd/__DARWIN_NFDBITS] |= (1<<(__fd % __DARWIN_NFDBITS)));
    // } while(0)

    // The index is our file descriptor divided by (32 * 8).
    idx := fd / (32 * 8)

    // The bit to set is the remainder from our file descriptor / (32 * 8)
    rem := fd % (32 * 8)

    fd_set.Bits[idx] |= (1 << rem)
}

func fdClear(fd_set *syscall.FdSet, fd uintptr) {
    // Read above for information.
    idx := fd / (32 * 8)
    rem := fd % (32 * 8)
    fd_set.Bits[idx] &= ^(1 << rem)
}

func packetReader(t *DarwinTunTap) {
    var n int
    var err error
    var fds syscall.FdSet
    packet := make([]byte, 65535)

    // Set value in fds.
    fdSet(&fds, t.file.Fd())

    for {
        // On Mac OS X, reading from the tun/tap device will do strange
        // things.  Use syscall.Select?

        // Wait forever for data to be ready.
        // FIXME: This doesn't work...
        //err = syscall.Select(1, &fds, nil, nil, nil)
        //if err != nil {
        //    log.Printf("Error in select() call: %s\n", err)
        //    continue
        //}

        // Actually read.
        n, err = t.file.Read(packet)
        if err == io.EOF {
            t.eof <- true
            return
        } else if err != nil {
            log.Printf("Error reading from tuntap: %s\n", err)
            <-time.After(1 * time.Second)
            continue
        }
        log.Printf("  got %d\n", n)

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
