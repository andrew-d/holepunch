import struct
import logging

import evergreen.tasks
from evergreen.lib import socket
from evergreen.queue import Queue, Empty

from .base import ClientBase, ConnectionError


log = logging.getLogger(__name__)
PORT = 44460


def _read_all(sock, length):
    chunks = []
    while length > 0:
        c = sock.recv(length)
        if len(c) == 0:
            return None
        chunks.append(c)
        length -= len(c)

    return b''.join(chunks)


def read_task(sock, queue):
    try:
        while True:
            lengthb = _read_all(sock, 2)
            if lengthb is None:
                log.warn("Failed reading length, stopping loop...")
                break

            length = struct.unpack("!H", lengthb)[0]
            packet = _read_all(sock, length)
            if packet is None:
                log.warn("Failed reading packet of length %d, stopping loop...",
                         length)
                break

            queue.put(packet, True)
    except socket.error:
        log.exception("Socket error in read loop, stopping loop")


def write_task(sock, queue):
    try:
        while True:
            packet = queue.get(True)

            length = struct.pack('!H', len(packet))
            sock.sendall(length)
            sock.sendall(packet)
    except socket.error:
        log.exception("Socket error in write loop, stopping...")


class TCPClient(ClientBase):
    @classmethod
    def connect_to(klass, address):
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

        return klass(s)

    def __init__(self, sock):
        self.sock = sock

        # Create queues.
        self.read_queue = Queue(1)
        self.write_queue = Queue(1)     # TODO: increase this one?

        # Start the read/write tasks.
        evergreen.tasks.spawn(read_task, sock, self.read_queue)
        evergreen.tasks.spawn(write_task, sock, self.write_queue)

    def get_packet(self, timeout=None):
        if timeout is None:
            return self.read_queue.get(True)
        else:
            try:
                return self.read_queue.get(True, timeout)
            except Empty:
                return None

    def send_packet(self, packet):
        self.write_queue.put(packet, True)

    @property
    def name(self):
        return "TCP"

    def close(self):
        self.sock.close()


def connect(server_addr):
    log.info("Attempting to create TCP transport...")
    try:
        return TCPClient.connect_to(server_addr)
    except ConnectionError:
        log.debug("Could not connect with TCP")
        return None


def listen(callback, *args, **kwargs):
    # Listen on all interfaces.
    s = None
    for res in socket.getaddrinfo(None, PORT, socket.AF_UNSPEC,
                                  socket.SOCK_STREAM, 0, socket.AI_PASSIVE):
        af, socktype, proto, canonname, sa = res
        try:
            s = socket.socket(af, socktype, proto)
        except socket.error as msg:
            s = None
            continue

        # Re-use bound addresses.
        s.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)

        try:
            s.bind(sa)
            s.listen(1)
        except socket.error as msg:
            log.warn("Can't listen on %s:%d: %r", sa[0], sa[1], msg)
            s.close()
            s = None
            continue

        break

    if s is None:
        log.error("Could not listen on any port (TCP)!")
        return

    log.info("Started listening on: %s:%d", *sa)

    while True:
        conn, addr = s.accept()
        client = TCPClient(conn)
        log.info("Client connected: %r", addr)
        evergreen.tasks.spawn(callback, client, *args, **kwargs)
