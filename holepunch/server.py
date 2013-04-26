"""
Usage: holepunch server [options]

Options:
    --methods METH      Methods to start (comma-seperated list of: tcp, udp,
                        icmp, dns).  Defaults to all of the methods.
"""
import os
import hmac
import hashlib
import logging

import evergreen.tasks
import evergreen.futures

from . import transports
from .common import forward_packets


log = logging.getLogger(__name__)


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
        t = evergreen.tasks.spawn(mod.listen, new_client, device, pwd)
        tasks.append(t)

    # Wait for all tasks
    log.info("Waiting for all transports to finish...")
    for t in tasks:
        t.join()


def new_client(conn, tun, pwd):
    # Send the challenge.
    nonce = os.urandom(32)

    # Send the nonce to the client.
    conn.send_packet(nonce)

    # Compute the expected response.
    hm = hmac.new(pwd, digestmod=hashlib.sha256)
    hm.update(nonce)
    response = hm.hexdigest()

    # Get the response, or time out after 10 seconds.
    resp = conn.get_packet(timeout=10.0)
    if resp is None:
        log.error("Authentication timed out, closing connection...")
        conn.close()
        return

    # Ok, compare the value.
    if resp != response:
        log.error("Authentication failed, closing connection.")
        conn.send_packet("failure")
        conn.close()
        return

    # Success!
    log.info("Client successfully authenticated")
    conn.send_packet("success")

    # Spawn a new task that forwards from the tun device --> client, and then
    # we become a task that forwards from the client to the tun device.
    evergreen.tasks.spawn(forward_packets, tun, conn, "tun", "client")
    forward_packets(conn, tun, "client", "tun")
