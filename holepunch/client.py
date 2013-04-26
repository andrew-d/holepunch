"""
Usage: holepunch client [options] <address>

Options:
    --methods METH      Methods to try
"""
import hmac
import hashlib
import logging

from evergreen.lib import socket

from . import transports


log = logging.getLogger(__name__)


def run(device, arguments):
    log.debug("Holepunching with server '%s'...", arguments['<address>'])

    # Try each method of connection.
    methods = arguments['--methods']
    if methods is None:
        methods = 'tcp,udp,icmp,dns'
    methods = [x.strip() for x in methods.split(',')]

    found = False
    for method in methods:
        log.info("Trying method %s...", method)
        mod = getattr(transports, method)

        # Try and create the transport.
        transport = mod.connect(arguments['<address>'])
        if not transport:
            continue

        # Test the transport.
        if test_transport(transport, arguments['--password']):
            log.info("Transport '%s' successfully connected!", method)
            found = True

    if found is False:
        log.error("Did not find a transport that works!")
        return


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
