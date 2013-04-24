// +build linux

package tuntap

import (
    "log"
    "os"
    "syscall"
    "unsafe"
)

func GetTuntapDevice() (*os.File, string, error) {
    name, err := getTuntapName()
    if err != nil {
        log.Printf("Error getting name: %s\n", err)
        return nil, "", err
    }
    log.Printf("Using name: %s\n", name)

    file, err := os.OpenFile("/dev/net/tun", os.O_RDWR, 0)
    if err != nil {
        log.Printf("Error opening file: %s\n", err)
        return nil, "", err
    }

    // Create the request to send.
    var req ifReq
    req.Flags = iffOneQueue | iffTun
    copy(req.Name[:15], name)

    // Send the request.
    _, _, serr := syscall.Syscall(syscall.SYS_IOCTL, file.Fd(), uintptr(syscall.TUNSETIFF), uintptr(unsafe.Pointer(&req)))
    if serr != 0 {
        log.Printf("Error with syscall: %s\n", err)
        return nil, "", err
    }

    return file, name, nil
}
