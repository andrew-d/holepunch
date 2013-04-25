import struct
import logging

from evergreen.lib import socket
from evergreen.queue import Queue, Empty

from .base import ClientBase, ConnectionError


log = logging.getLogger(__name__)
PORT = 44460


def _read_all(sock, length):
    buff = bytearray(length)
    mv = memoryview(buff)
    offset = 0
    while length > 0:
        l = sock.recv_into(mv[offset:])
        if l == 0:
            return None

        offset += l
        length -= l

    return buff


def read_task(sock, queue):
    while True:
        lengthb = _read_all(2)
        if lengthb is None:
            log.warn("Failed reading length, stopping loop...")
            break

        length = struct.unpack("!H", lengthb)[0]
        packet = _read_all(length)
        if packet is None:
            log.warn("Failed reading packet of length %d, stopping loop...",
                     length)
            break

        queue.put(packet, True)


def write_task(sock, queue):
    while True:
        packet = queue.get(True)

        length = struct.pack('!H', len(packet))
        sock.sendall(length)
        sock.sendall(packet)


class TCPClient(ClientBase):
    def __init__(self, address):
        s = None
        for res in socket.getaddrinfo(address, PORT, socket.AF_UNSPEC,
                                      socket.SOCK_STREAM):
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
                s.settimeout(0.0)
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

        self.sock = s

        # Create queues.
        self.read_queue = Queue(1)
        self.write_queue = Queue(1)     # TODO: increase this one?

        # Start the read/write tasks.
        evergreen.tasks.spawn(read_task, self.sock, self.read_queue)
        evergreen.tasks.spawn(write_task, self.sock, self.write_queue)

    def get_packet(self, timeout=None):
        if timeout is None:
            return self.queue.get(True)
        else:
            try:
                return self.queue.get(False, timeout)
            except Empty:
                return None

    def send_packet(self, packet):
        self.write_queue.put(packet, True)

    @property
    def name(self):
        return "TCP"


def connect(server_addr):
    log.info("Attempting to create TCP transport...")
    try:
        return TCPClient(server_addr)
    except ConnectionError:
        log.debug("Could not connect with TCP")
        return None


def listen():
    pass
