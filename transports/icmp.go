package transports

import (
    "bytes"
    "encoding/binary"
    "net"
)

type ICMPHeader struct {
    Type     int8
    Code     int8
    Checksum uint16
    ID       uint16
    Sequence int16
}

type ICMPPacketClient struct {
    conn     *net.IPConn
    incoming chan []byte
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
    return nil, nil
}

func (c *ICMPPacketClient) SendPacket(pkt []byte) error {
    data := serializeICMP(pkt)
    c.conn.Write(data)
}

func (c *ICMPPacketClient) PacketChannel() chan []byte {
    return c.incoming
}

func (c *ICMPPacketClient) Describe() string {
    return "ICMPPacketClient"
}

func (c *ICMPPacketClient) Close() {
    // TODO: do we need to close anything?
}

func serializeICMP(data []byte) ([]byte, err) {
    // TODO: Choose special values for our ICMP header.
    hdr := ICMPHeader{8, 0, 0, 1234, 1}

    // Calculate the checksum for our header.
    chk, err := getChecksum(hdr, pkt)
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
