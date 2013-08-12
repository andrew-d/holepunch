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
    file     *os.File
    name     string
    packets  chan []byte
    finished chan bool
    exit     chan bool
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

    packets := make(chan []byte)
    finished := make(chan bool)
    exit := make(chan bool)

    tuntap := &DarwinTunTap{tuntapDev, name, packets, finished, exit}
    return tuntap, nil
}

func (t *DarwinTunTap) Start() {
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
    var exit bool
    packet := make([]byte, 65535)
    fds := new(syscall.FdSet)
    fd := int(t.file.Fd())
    FD_SET(fd, fds)

    for {
        // On Mac OS X, reading from the tun/tap device will do strange
        // things.  We need to use syscall.Select.
        syscall.Select(fd+1, fds, nil, nil, nil)

        // We need to check whether we're to exit here - before the .Read() call
        // below, since the syscall above could have been terminated by the
        // closure of the associated fd.  We do a non-blocking select that will
        // fall through to the default case if there is nothing to read from our
        // exit channel to determine this.
        select {
        case exit = <-t.exit:
        default:
        }
        if exit {
            log.Printf("Exit signal received\n")
            break
        }

        n, err := t.file.Read(packet)
        if err == io.EOF {
            break
        } else if err != nil {
            log.Printf("Error reading from tuntap: %s\n", err)

            // This wait is to stop us from getting stuck in an infinite loop
            // of reads that all error, and consuming 100% CPU forever.
            <-time.After(100 * time.Millisecond)
            continue
        }

        t.packets <- packet[0:n]
    }

    // This needs to be the last thing in the function.
    log.Printf("Done")
    t.finished <- true
}

func (t *DarwinTunTap) RecvChannel() chan []byte {
    return t.packets
}

func (t *DarwinTunTap) EOFChannel() chan bool {
    return t.finished
}

func (t *DarwinTunTap) Write(pkt []byte) error {
    _, err := t.file.Write(pkt)
    return err
}

func (t *DarwinTunTap) Name() string {
    return t.name
}

func (t *DarwinTunTap) Close() {
    // We run the syscall.Select (above) with no timeout, so we need to force
    // the call to abort.  The easiest way to do this is close the fd.
    t.file.Close()
    t.file = nil

    // Tell the tuntap reader to exit...
    // TODO: I think there might be a race condition here,
    // we might need to make this a 1-sized channel and put
    // the channel call before the file.Close()
    t.exit <- true

    // ... and wait for it to finish.
    <-t.finished
}
