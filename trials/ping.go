package main

import (
    "fmt"
    "log"
    "net"
    "encoding/binary"
    "encoding/hex"
    "bytes"
)

type ICMPHeader struct {
    Type     int8
    Code     int8
    Checksum uint16
    ID       uint16
    Sequence int16
}

func main() {
    remote := "127.0.0.1"

    addr, err := net.ResolveIPAddr("ip", remote)
    if err != nil {
        log.Printf("Error resolving address: %s\n", err)
        return
    }

    conn, err := net.DialIP("ip:icmp", nil, addr)
    if err != nil {
        log.Printf("Error dialing ICMP: %s\n", err)
        return
    }
    defer conn.Close()

    log.Printf("Connected to: %s\n", remote)

    // Make ICMP header.
    hdr := ICMPHeader{8, 0, 0, 1234, 1}

    // Calculate checksum over header + data
    chk, err := getChecksum(hdr)
    if err != nil {
        log.Printf("Error calculating checksum: %s\n", err)
        return
    }

    // Add checksum.
    log.Printf("Checksum is: %d\n", chk)
    chk = 0xF32C
    hdr.Checksum = chk

    // Send it.
    buf := new(bytes.Buffer)
    err = binary.Write(buf, binary.BigEndian, hdr)
    if err != nil {
        log.Printf("Error encoding buffer: %s\n", err)
        return
    }

    log.Println("Sending request...")
    conn.Write(buf.Bytes())

    b := make([]byte, 65535)
    amt, ip, err := conn.ReadFrom(b)
    if err != nil {
        log.Printf("Error receiving: %s\n", err)
        return
    }

    log.Printf("Got %d bytes from %s\n", amt, ip)
    fmt.Println(hex.Dump(b[0:amt]))

    // Decode to ICMP header
    buf = bytes.NewBuffer(b[0:amt])
    err = binary.Read(buf, binary.BigEndian, &hdr)
    if err != nil {
        log.Printf("Error decoding response: %s\n", err)
        return
    }

    log.Printf("Type = %d, Code = %d, Checksum = %d, ID = %d, Sequence = %d",
               hdr.Type, hdr.Code, hdr.Checksum, hdr.ID, hdr.Sequence)
}

func getChecksum(hdr ICMPHeader) (uint16, error) {
    buf := new(bytes.Buffer)
    err := binary.Write(buf, binary.BigEndian, hdr)
    if err != nil {
        return 0, err
    }
    arr := buf.Bytes()

    var sum uint32
    countTo := (len(arr) / 2) * 2

    for i := 0; i < countTo; i += 2 {
        sum += (uint32)(arr[i + 1]) * 256 + (uint32)(arr[i])
    }

    if countTo < len(arr) {
        sum += (uint32)(arr[len(arr) - 1])
    }

    sum = (sum >> 16) + (sum & 0xFFFF)
    sum = sum + (sum >> 16)
    answer := (uint16)((^sum) & 0xFFFF)
    answer = answer >> 8 | ((answer << 8) | 0xFF00)

    return answer, nil
}
