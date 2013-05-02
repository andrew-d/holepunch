import time
import struct
import socket
import logging
import threading

from ..six.moves import queue
from .base import ClientBase, ConnectionError, SocketDisconnected


log = logging.getLogger(__name__)
PORT = 44461
TRANSMIT_TIMEOUT = 1.0
CONNECTION_TIMEOUT = 10.0


# Notes:
# ------
# We implement a very simple reliable data transfer algorithm on top of the
# underlying transport.  We prepend each packet with the following header:
#   | type  | seq | flags | data .... |
# "type" is a 1-byte field, and is one of the following:
#   0x00        Data packet
#   0x01        Data acknowledgement
#   0x02        Keep-alive (no need for acknowledgement)
#
# "seq" is a 16-bit number that represents the current index of the packet. A
# different index is used for sends and receives.
# "flags" is a 1-byte number reserved for future use.
# Packets that are to be sent are appended to a fixed-size send queue, and will
# be stored on the internal buffer.  A background thread will pull packets off
# the queue and send them, waiting for an acknowledgement packet from the
# server before transmitting any further packets.  If an acknowledgement is not
# received within a certain number of seconds, the packet is retransmitted.  If
# no acknowledgement is received within a larger number of seconds, the
# connection is classified as dropped.  Furthermore, we use a simple keep-alive
# protocol that simply sends a single packet every N seconds, where N is one-
# quarter of the connection timeout.
# On the receiving end, whenever a packet is received, we immediately send the
# corresponding acknowledgement packet.
#
# TODO: Generalize to ICMP and DNS too
# TODO: use more efficient algorithm


rel_header = struct.Struct("!BHB")
ID_DATA         = 0x00
ID_ACK          = 0x01
ID_KEEPALIVE    = 0x02


class ReliablePacketTransport(object):
    def __init__(self, underlying):
        self.underlying = underlying
        self.recv_queue = queue.Queue(5)
        self.send_queue = queue.Queue(5)
        self.dropped = False

        # Sequence numbers.
        self.recv_seq = 0
        self.send_seq = 0
        self.send_seq_evt = threading.Event()

        # Start threads.

    def send_packet(self, packet):
        pass

    def get_packet(self, timeout=None):
        pass

    def close(self):
        self.underlying.close()

    def _internal_sender(self):
        keep_alive = CONNECTION_TIMEOUT / 4.0
        last_send = self.send_seq

        while True:
            # If we've dropped connection, finish this thread.
            if self.dropped:
                break

            # Default to sending a keep-alive.
            pkt = b'\x02\x00\x00\x00'

            # Check our sending sequence number.  We essentially want to wait
            # until the other end has acknowledged a given sequence number

            # Get a packet from the queue.  Note that we set the timeout to be
            # the keep-alive time, so that if the get operation times out, we
            # catch it below and send a keep-alive packet.
            pkt = self.send_queue.get(False, keep_alive)

            self.underlying.send_packet(pkt)

    def _internal_receiver(self):
        while True:
            # Get packet, timing out properly.
            pkt = self.underlying.get_packet(timeout=CONNECTION_TIMEOUT)
            if pkt is None:
                self.dropped = True
                break

            # Decode the packet.
            header, data = pkt[:4], pkt[4:]
            type, seq, flags = rel_header.unpack(header)

            # If this is a keep-alive, ignore it - it just stopped us from
            # hitting the timeout case above.
            if type == ID_KEEPALIVE:
                continue

            # If this is an acknowledgement, then it is the other end telling
            # us that it got one of our packets.  We update the send sequence
            # number, so that our sending thread can proceed.
            elif type == ID_ACK:
                self.send_seq = seq
                self.send_seq_evt.set()
                continue

            # If we get here, it's should be an ID_DATA packet.
            elif type != ID_DATA:
                log.warn("Invalid packet data type: %d", type)
                continue

            # This is a data packet.  The sequence number should be what we
            # expect, and, if not, we discard it and re-ack the previous
            # sequence number.
            if seq != self.recv_seq:
                data = rel_header.pack(ID_ACK, self.recv_seq, 0)
                valid = False
            else:
                # Acknowledge the receive.  Note that we must do this BEFORE
                # putting it on the queue, in case the queue blocks.
                data = rel_header.pack(ID_ACK, seq, 0)
                valid = True

            # Send the correct packet.
            self.underlying.send_packet(data)

            # If it's valid, put on queue and update our receiving sequence
            # number.  The queue.put operation can block.
            if valid:
                self.recv_queue.put(data)
                self.recv_seq = seq


class UDPClient(ClientBase):
    @classmethod
    def connect_to(klass, address):
        s = None
        for res in socket.getaddrinfo(address, PORT, socket.AF_UNSPEC,
                                      socket.SOCK_DGRAM):
            af, socktype, proto, canonname, sa = res
            try:
                s = socket.socket(af, socktype, proto)
            except socket.error as msg:
                s = None
                continue

            try:
                log.debug("Trying with sockaddr: %r", sa)
                s.settimeout(2.0)
                s.connect(sa)
                s.settimeout(None)
            except socket.error as msg:
                s.close()
                s = None
                continue

            break

        if s is None:
            log.warn("Cannot connect to host %s:%d", address, PORT)
            raise ConnectionError("Could not connect to host!")
        else:
            log.info("Connected to host %s:%d", address, PORT)

        return klass(s, (address, PORT))

    def __init__(self, sock, addr):
        self.sock = sock
        self.addr = addr

    def get_packet(self, timeout=None):
        # Just read and return a whole packet.  Reliability is at another level
        data, addr = self.sock.recvfrom(65535)
        return data

    def send_packet(self, packet_data):
        self.sock.sendto(packet_data, self.addr)

    def close(self):
        self.sock.close()

    @property
    def name(self):
        return "UDP"



def connect(address):
    log.info("Attempting to create UDP transport...")
    try:
        return UDPClient.connect_to(address)
    except ConnectionError:
        log.debug("Could not connect with TCP")
        return None


def listen(callback, *args, **kwargs):
    pass
