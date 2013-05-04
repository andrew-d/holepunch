"""
Usage: holepunch client [options] <address>

Options:
    --methods METH      Methods to try
"""
import hmac
import hashlib
import logging

import evergreen

from . import transports
from .common import forward_packets, wait_for_multiple_tasks
from .config import set_route_gateway, get_gateway_for_ip


log = logging.getLogger(__name__)


# Architecture:
# -------------
# Unlike the server, the client is pretty simple :-)  We start two tasks -
# one that will forward packets from the TUN device to the server, and then one
# to forward from the server to the TUN device.  Our main task will then wait
# for these two tasks to exit (see server.py for comment regarding signals).


def run(device, arguments):
    address = arguments['<address>']
    log.debug("Holepunching with server '%s'...", address)

    # Try each method of connection.
    methods = arguments['--methods']
    if methods is None:
        methods = 'tcp,udp,icmp,dns'
    methods = [x.strip() for x in methods.split(',')]

    found = False
    conn = None
    for method in methods:
        log.info("Trying method %s...", method)
        mod = getattr(transports, method)

        # Try and create the transport.
        transport = mod.connect(address)
        if not transport:
            continue

        # Test the transport.
        pwd = arguments['--password'] or ''
        if test_transport(transport, pwd):
            log.info("Transport '%s' successfully connected!", method)
            found = True
            conn = transport
            break

    if found is False:
        log.error("Did not find a transport that works!")
        return

    # Get the existing route to our holepunch server.
    existing_route = get_gateway_for_ip(address)
    if existing_route is None:
        log.warn("Cannot set up routes - you should do this manually!  Set:\n"
                 "- A route for 0.0.0.0/0 through 10.93.0.1\n"
                 "- A route for %s through your default gateway", address)
    else:
        # Set one route that forwards all traffic (for the whole internet)
        # through our gateway IP.
        set_route_gateway(device.name, '0.0.0.0/0', '10.93.0.1')

        # Set another route that forwards all traffic for our actual gateway
        # through the previous default route.
        set_route_gateway(device.name, address, existing_route)

    # Forward packets.
    t1 = evergreen.tasks.spawn(forward_packets, conn, device)
    t2 = evergreen.tasks.spawn(forward_packets, device, conn)

    # Wait for all tasks.  Note that we have a timeout here so we don't stop
    # signals from being processed.
    log.info("Waiting for forwarding tasks to finish...")
    try:
        wait_for_multiple_tasks([t1, t2])
    except KeyboardInterrupt:
        pass

    log.info("Client is finished")


def test_transport(transport, password):
    # Read the nonce from the transport.
    nonce = transport.get_packet()

    # Compute the HMAC of this challenge
    hm = hmac.new(password, digestmod=hashlib.sha256)
    hm.update(nonce)

    # Send the response back.
    transport.send_packet(hm.hexdigest())

    # Get a packet.
    ret = transport.get_packet()
    if ret == 'success':
        return True
    elif ret == 'failure':
        return False
    else:
        return False
