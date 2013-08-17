package transports

import (
    "bytes"
    "encoding/binary"
    "net"
    "log"
)

type ICMPHeader struct {
    Type     int8
    Code     int8
    Checksum uint16
    ID       uint16
    Sequence int16
}

type ICMPPacketClient struct {
    conn         *net.IPConn
    send_ch      chan []byte
    recv_ch      chan []byte
    old_icmp_val bool
}

/* Architecture:
 * -------------
 * We hold open one single socket that receives all ICMP packets destined for
 * the host computer.  We maintain a map that connects the remote IP address of
 * a packet to the client associated with it.  For each incoming packet, we
 * pass the packet to the associated client, or, if it isn't associated with a
 * client, we pass it to the accepting channel.  The accepting goroutine will
 * then either accept the new client, or respond to the ping packet according
 * to the settings specified.
 *
 *
 *
 */

func NewICMPPacketClient(server string) (*ICMPPacketClient, error) {
    addr, err := net.ResolveIPAddr("ip", server)
    if err != nil {
        return nil, err
    }

    conn, err := net.DialIP("ip:icmp", nil, addr)
    if err != nil {
        return nil, err
    }

    // TODO: start the read process.
    send_ch := make(chan []byte)
    recv_ch := make(chan []byte)
    client := &ICMPPacketClient{conn, send_ch, recv_ch, true}

    // TODO: cross-platform!
    old_val, err := ChangeRespondToPings(false)
    if err != nil {
        client.old_icmp_val = old_val
    }

    return nil, nil
}

func (c *ICMPPacketClient) SendPacket(pkt []byte) error {
    data, err := serializeICMP(pkt)
    if err != nil {
        return err
    }
    c.conn.Write(data)
    return nil
}

func (c *ICMPPacketClient) SendChannel() chan []byte {
    return c.send_ch
}

func (c *ICMPPacketClient) RecvChannel() chan []byte {
    return c.recv_ch
}

func (c *ICMPPacketClient) IsReliable() bool {
    return false
}

func (c *ICMPPacketClient) Describe() string {
    return "ICMPPacketClient"
}

func (c *ICMPPacketClient) Close() {
    _, err := ChangeRespondToPings(c.old_icmp_val)
    if err != nil {
        log.Printf("Error restoring ICMP setting: %s\n", err)
    }
}

func serializeICMP(data []byte) ([]byte, error) {
    // TODO: Choose special values for our ICMP header.
    hdr := ICMPHeader{8, 0, 0, 1234, 1}

    // Calculate the checksum for our header.
    chk, err := getChecksum(hdr, data)
    if err != nil {
        return nil, err
    }
    hdr.Checksum = chk

    // Serialize it all.  Note that network order = big endian.
    buf := new(bytes.Buffer)
    err = binary.Write(buf, binary.BigEndian, hdr)
    if err != nil {
        return nil, err
    }
    arr := append(buf.Bytes(), data...)

    return arr, nil
}

// Calculates the ICMP checksum.
func getChecksum(hdr ICMPHeader, data []byte) (uint16, error) {
    buf := new(bytes.Buffer)
    err := binary.Write(buf, binary.BigEndian, hdr)
    if err != nil {
        return 0, err
    }
    arr := append(buf.Bytes(), data...)

    var sum uint32
    countTo := (len(arr) / 2) * 2

    // Sum as if we were iterating over uint16's
    for i := 0; i < countTo; i += 2 {
        p1 := (uint32)(arr[i+1]) * 256
        p2 := (uint32)(arr[i])
        sum += p1 + p2
    }

    // Potentially sum the last byte
    if countTo < len(arr) {
        sum += (uint32)(arr[len(arr)-1])
    }

    // Fold into 16 bits.
    sum = (sum >> 16) + (sum & 0xFFFF)
    sum = sum + (sum >> 16)

    // Take the 1's complement, and swap bytes.
    answer := ^((uint16)(sum & 0xFFFF))
    answer = (answer >> 8) | ((answer << 8) & 0xFF00)

    return answer, nil
}
