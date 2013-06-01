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
    log.Printf("Type = %d, Code = %d, Checksum = %d, ID = %d, Sequence = %d",
               hdr.Type, hdr.Code, hdr.Checksum, hdr.ID, hdr.Sequence)
    data := []byte("foobar")

    // Calculate checksum over header + data
    chk, err := getChecksum(hdr, data)
    if err != nil {
        log.Printf("Error calculating checksum: %s\n", err)
        return
    }

    // Add checksum.
    log.Printf("Checksum is: 0x%x\n", chk)
    hdr.Checksum = chk

    // Send it.
    buf := new(bytes.Buffer)
    err = binary.Write(buf, binary.BigEndian, hdr)
    if err != nil {
        log.Printf("Error encoding buffer: %s\n", err)
        return
    }
    arr := append(buf.Bytes(), data...)

    log.Println("Sending request...")
    conn.Write(arr)

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

func getChecksum(hdr ICMPHeader, data []byte) (uint16, error) {
    buf := new(bytes.Buffer)
    err := binary.Write(buf, binary.BigEndian, hdr)
    if err != nil {
        return 0, err
    }
    arr := append(buf.Bytes(), data...)

    fmt.Printf(hex.Dump(arr))

    var sum uint32
    countTo := (len(arr) / 2) * 2

    // Sum as if we were iterating over uint16's
    for i := 0; i < countTo; i += 2 {
        p1 := (uint32)(arr[i + 1]) * 256
        p2 := (uint32)(arr[i])
        sum += p1 + p2
    }

    // Potentially sum the last byte
    if countTo < len(arr) {
        sum += (uint32)(arr[len(arr) - 1])
    }

    // Fold into 16 bits.
    sum = (sum >> 16) + (sum & 0xFFFF)
    sum = sum + (sum >> 16)

    // Take the 1's complement, and swap bytes.
    answer := ^((uint16)(sum & 0xFFFF))
    answer = (answer >> 8) | ((answer << 8) & 0xFF00)

    return answer, nil
}
