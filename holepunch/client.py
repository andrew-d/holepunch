import logging

from . import transports


log = logging.getLogger(__name__)


def client(device, arguments):
    log.info("Holepunching with server '%s'...", arguments['<address>'])

    # Try each method of connection.
    for method in ['tcp', 'udp', 'icmp', 'dns']:
        log.info("Trying method %s...", method)
        mod = getattr(transports, method)

        # Try and create the transport.
        transport = mod.new(arguments['<address>'])
        if not transport:
            continue

        # Test the transport.
