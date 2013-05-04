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

import evergreen

from . import transports
from .transports.base import SocketDisconnected
from .common import forward_packets, wait_for_multiple_tasks


log = logging.getLogger(__name__)


# Architecture:
# -------------
# We start a new task for each transport method.  These tasks will each
# call the (blocking) accept_client() method on the transport.  Each time this
# method returns, it will return a client connection, in the form of an object
# implementing ClientBase.  The server will then spawn another task that will
# read packets from this client and forward them to the TUN device.
# Furthermore, the client connection will be added to a global list.  This list
# will be used in a final task that will forward all incoming packets from
# the TUN device to all connected clients.  Note that this is somewhat
# inefficient, but we don't want to do routing here.  In summary, the tasks
# we use are:
#   - 1 task for each transport method (currently 4)
#   - 1 task that will forward any packets from the TUN device to all
#     connected clients
#   - 1 task for each individual client that is connected, that will forward
#     packets from the client to the TUN device
#   - The main task, which will wait for all tasks to exit.  Note that we
#     do this to catch signals.
# Total tasks: 6 + number of connected clients


def run(device, arguments):
    # Get methods.
    methods = arguments['--methods']
    if methods is None:
        methods = 'tcp,udp,icmp,dns'
    methods = [x.strip() for x in methods.split(',')]

    # Password.
    pwd = arguments['--password'] or ''

    # Start methods.
    tasks = []
    for method in methods:
        log.info("Starting transport %s...", method)
        mod = getattr(transports, method)

        t = evergreen.tasks.Task(mod.listen, name='transport_%s' % (method,),
                                 args=(new_client, device, pwd)
                                 )
        t.start()
        tasks.append(t)

    # Wait for all tasks
    log.info("Waiting for all transports to finish...")
    try:
        wait_for_multiple_tasks(tasks)
    except KeyboardInterrupt:
        pass

    log.info("Server is finished")


def const_compare(a, b):
    """A simple constant-time comparison function."""
    if len(a) != len(b):
        return False

    res = 0
    for x, y in zip(a, b):
        res |= ord(x) ^ ord(y)

    return res == 0


def new_client(conn, addr, tun, pwd):
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

    # Spawn a new task that forwards from the tun device --> client, and then
    # we become a task that forwards from the client to the tun device.
    evergreen.tasks.spawn(forward_packets, tun, conn)
    forward_packets(conn, tun)
