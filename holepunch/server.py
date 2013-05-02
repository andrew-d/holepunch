"""
Usage: holepunch server [options]

Options:
    --methods METH      Methods to start (comma-seperated list of: tcp, udp,
                        icmp, dns).  Defaults to all of the methods.
"""
import os
import hmac
import socket
import hashlib
import logging
import threading

from . import transports
from .transports.base import SocketDisconnected
from .common import forward_packets


log = logging.getLogger(__name__)


# Architecture:
# -------------
# We start a new thread for each transport method.  These threads will each
# call the (blocking) accept_client() method on the transport.  Each time this
# method returns, it will return a client connection, in the form of an object
# implementing ClientBase.  The server will then spawn another thread that will
# read packets from this client and forward them to the TUN device.
# Furthermore, the client connection will be added to a global list.  This list
# will be used in a final thread that will forward all incoming packets from
# the TUN device to all connected clients.  Note that this is somewhat
# inefficient, but we don't want to do routing here.  In summary, the threads
# we use are:
#   - 1 thread for each transport method (currently 4)
#   - 1 thread that will forward any packets from the TUN device to all
#     connected clients
#   - 1 thread for each individual client that is connected, that will forward
#     packets from the client to the TUN device
#   - The main thread, which will wait for all threads to exit.  Note that we
#     do this because Python will only deliver exceptions to the main thread,
#     so we don't want to block this thread for too long.
# Total threads: 6 + number of connected clients


def run(device, arguments):
    # Get methods.
    methods = arguments['--methods']
    if methods is None:
        methods = 'tcp,udp,icmp,dns'
    methods = [x.strip() for x in methods.split(',')]

    # Password.
    pwd = arguments['--password'] or ''

    # Start methods.
    threads = []
    for method in methods:
        log.info("Starting transport %s...", method)
        mod = getattr(transports, method)

        t = threading.Thread(target=mod.listen,
                             args=(new_client, device, pwd),
                             name="transport_%s" % (method,))
        t.daemon = True
        t.start()
        threads.append(t)

    # Wait for all threads
    log.info("Waiting for all transports to finish...")
    try:
        while True:
            if len(threads) == 0:
                break

            # Wait for the thread.
            t = threads[0]
            t.join(2.0)

            # If it has exited, remove it from our list.
            if not t.is_alive():
                log.debug("Thread %s has exited", t.name)
                threads.remove(t)
    except KeyboardInterrupt:
        pass

    log.info("Server is finished")


def const_compare(a, b):
    """A simple constant-time comparison function."""
    if len(a) != len(b):
        return False

    res = 0
    for x, y in zip(a, b):
        res |= x ^ y

    return res == 0


def new_client(conn, tun, pwd):
    # Send the challenge.
    nonce = os.urandom(32)

    # Send the nonce to the client.
    conn.send_packet(nonce)

    # Compute the expected response.
    hm = hmac.new(pwd, digestmod=hashlib.sha256)
    hm.update(nonce)
    expected = hm.hexdigest()

    # Get the response, or time out after 10 seconds.
    try:
        resp = conn.get_packet(timeout=10.0)
    except (socket.error, SocketDisconnected):
        resp = None

    if resp is None:
        log.error("Authentication timed out, closing connection...")
        conn.close()
        return

    # Ok, compare the value.
    if not const_compare(resp, expected):
        log.error("Authentication failed, closing connection.")
        conn.send_packet("failure")
        conn.close()
        return

    # Success!
    log.info("Client successfully authenticated")
    conn.send_packet("success")

    # Spawn a new thread that forwards from the tun device --> client, and then
    # we become a thread that forwards from the client to the tun device.
    t = threading.Thread(target=forward_packets,
                         name="tun_to_conn",
                         args=(tun, conn, "tun", "client")
                         )
    t.daemon = True
    t.start()

    forward_packets(conn, tun, "client", "tun")
