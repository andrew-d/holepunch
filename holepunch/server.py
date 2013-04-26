"""
Usage: holepunch server [options]

Options:
    --methods METH      Methods to start (comma-seperated list of: tcp, udp,
                        icmp, dns).  Defaults to all of the methods.
"""
import logging

from . import transports


log = logging.getLogger(__name__)


def run(device, arguments):
    # Get methods.
    methods = arguments['--methods']
    if methods is None:
        methods = 'tcp,udp,icmp,dns'
    methods = [x.strip() for x in methods.split(',')]

    # Start methods.
    for method in methods:
        log.info("Starting transport %s...", method)
        mod = getattr(transports, method)
        mod.listen()
