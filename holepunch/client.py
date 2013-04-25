"""
Usage: holepunch client [options] <address>

Options:
    --methods METH      Methods to try
"""
import logging

from . import transports


log = logging.getLogger(__name__)


def run(device, arguments):
    log.debug("Holepunching with server '%s'...", arguments['<address>'])

    # Try each method of connection.
    for method in ['tcp', 'udp', 'icmp', 'dns']:
        log.info("Trying method %s...", method)
        mod = getattr(transports, method)

        # Try and create the transport.
        transport = mod.new(arguments['<address>'])
        if not transport:
            continue

        # Test the transport.
        if test_transport(transport):
            log.info("Transport '%s' successfully connected!", method)


def test_transport(transport):
    pass
