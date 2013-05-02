import time
import select
import socket
import struct
import logging
import threading

from .base import ClientBase, ConnectionError, SocketDisconnected


log = logging.getLogger(__name__)
PORT = 44460


class TimeoutSocketWrapper(object):
    def __init__(self, socket):
        self.sock = socket

    def read_bytes(self, num_bytes, timeout=None):
        # Default to infinite timeout.
        if timeout is None:
            timeout = float('inf')

        return self._internal_read(num_bytes, timeout)

    def _internal_read(self, length, timeout):
        # Firstly, get the absolute timeout.
        abs_timeout = time.time() + timeout

        # Now, loop while we still have time left.
        chunks = []
        while True:
            # If we have no more length to read, we're done.
            if length == 0:
                break

            # If we're past the absolute time, we're done.
            now = time.time()
            if now > abs_timeout:
                log.debug("Read timed out")
                return None

            # Wait for an available read for the remainder of the time.  Note
            # that the select() call can't take an infinite value, so we
            # special-case this to set the timeout to None, representing the
            # infinite value.
            if abs_timeout == float('inf'):
                time_left = None
            else:
                time_left = abs_timeout - now

            rlist, wlist, xlist = select.select([self.sock], [], [],
                                                time_left)

            # Check if we actually got some data.  If not, we timed out.  Note
            # that we don't catch that case - we just return to the top of the
            # loop and let the code there handle it.
            if len(rlist) > 0:
                c = self.sock.recv(length)
                if len(c) == 0:
                    log.warn("Socket read returned None!")
                    raise SocketDisconnected()

                chunks.append(c)
                length -= len(c)

        return b''.join(chunks)

    def sendall(self, *args, **kwargs):
        return self.sock.sendall(*args, **kwargs)

    def send(self, *args, **kwargs):
        return self.sock.send(*args, **kwargs)

    def recv(self, *args, **kwargs):
        return self.sock.recv(*args, **kwargs)

    def close(self):
        self.sock.close()


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
        self.sock = TimeoutSocketWrapper(sock)

    def get_packet(self, timeout=None):
        # Read the length
        lengthb = self.sock.read_bytes(2, timeout=timeout)
        if lengthb is None:
            log.warn("Failed reading length...")
            return None

        # Get the length as an integer, then read the data.
        length = struct.unpack("!H", lengthb)[0]
        packet = self.sock.read_bytes(length, timeout=timeout)
        if packet is None:
            log.warn("Failed reading packet of length %d...", length)
            return None

        return packet

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

    i = 0
    threads = []

    while True:
        conn, addr = s.accept()
        client = TCPClient(conn)
        log.info("Client connected: %s:%d", addr[0], addr[1])
        i += 1

        # Spawn a new thread
        thread_args = list(args)
        thread_args.insert(0, client)

        t = threading.Thread(target=callback,
                             name="tcp_client_%d" % (i,),
                             args=thread_args,
                             kwargs=kwargs)

        t.daemon = True
        t.start()
        threads.append(t)
