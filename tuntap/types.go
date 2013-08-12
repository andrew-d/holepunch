// +build linux
package tuntap

// Types for TUNTAP stuff on linux
const (
    flagTruncated = 0x1

    iffTun      = 0x1
    iffTap      = 0x2
    iffNoPI     = 0x1000
    iffOneQueue = 0x2000
)

type ifReq struct {
    Name  [0x10]byte
    Flags uint16
    pad   [0x28 - 0x10 - 2]byte
}

type Device interface {
    Start()
    RecvChannel() chan []byte
    EOFChannel() chan bool
    Write(pkt []byte) error
    Name() string
    Close()
}
