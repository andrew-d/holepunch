import time
import select
import struct
import logging

import evergreen
from evergreen.lib import socket

from .base import ClientBase, ConnectionError, SocketDisconnected


log = logging.getLogger(__name__)
PORT = 44460


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

    def _read_all(self, length):
        chunks = []
        while length > 0:
            d = self.sock.recv(length)
            if d is None:
                return None
            chunks.append(d)
            length -= len(d)

        return b''.join(chunks)

    def get_packet(self, timeout=None):
        with evergreen.timeout.Timeout(timeout, exception=False):
            # Read the length
            lengthb = self._read_all(2)
            if lengthb is None:
                log.warn("Failed reading length...")
                return None

            # Get the length as an integer, then read the data.
            length = struct.unpack("!H", lengthb)[0]
            packet = self._read_all(length)
            if packet is None:
                log.warn("Failed reading packet of length %d...", length)
                return None

            return packet

        return None

    def send_packet(self, packet):
        length = struct.pack('!H', len(packet))
        self.sock.sendall(length)
        self.sock.sendall(packet)

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


# This trampoline ensures that the client connection is closed after each
# task is finished with it.
def _new_client_trampoline(callback, client, addr, *args, **kwargs):
    try:
        callback(client, addr, *args, **kwargs)
    finally:
        client.close()


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

    try:
        while True:
            conn, addr = s.accept()
            client = TCPClient(conn)
            log.info("Client connected: %s:%d", addr[0], addr[1])

            # Spawn a new task to process this client.
            evergreen.tasks.spawn(_new_client_trampoline, callback, client, addr,
                                  *args, **kwargs)
    finally:
        s.close()
