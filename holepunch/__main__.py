"""Simple, low-configuration VPN

Usage:
    holepunch [options] client [<args>...]
    holepunch [options] server [<args>...]
    holepunch --version
    holepunch (-h | --help)

Options:
    -h --help       Show this screen.
    --version       Show version.
    -q, --quiet     Only output warnings and errors.
    -v ,--verbose   Output debug messages (useful for troubleshooting
                    connection problems).
    --password PASS The password to use for authentication.

"""
import logging

from docopt import docopt

from .config import set_interface_properties
from .tuntap import TunTapDevice
from .log import setup_logging
from . import client, server


log = logging.getLogger(__name__)


def main():
    args = docopt(__doc__, version='Holepunch v0.0.1', options_first=True)

    # Set up logging.
    if args['--quiet']:
        level = logging.WARN
    elif args['--verbose']:
        level = logging.DEBUG
    else:
        level = logging.INFO
    setup_logging(level)

    # Depending on the command selected, we pick the appropriate module that we
    # use for all future calls.
    if args['client']:
        cmd = 'client'
        mod = client
    else:
        cmd = 'server'
        mod = server

    # Parse arguments from the appropriate module.
    argv = [cmd] + args['<args>']
    sub_args = docopt(mod.__doc__, argv=argv)
    for i in ['--password']:
        sub_args[i] = args[i]

    # Set up TUN device - we need this for both server and client.
    log.info("Creating TUN device...")
    dev = TunTapDevice()

    # Configure the device.
    log.info("Configuring the TUN device...")
    if args['client']:
        set_interface_properties(dev.name, '10.93.0.2', '10.93.0.1')
    else:
        set_interface_properties(dev.name, '10.93.0.1', '10.93.0.1')

    # Start the device now that we've configured it.
    dev.setup()

    # Run the right thing.
    mod.run(dev, sub_args)


if __name__ == "__main__":
    try:
        main()
    except KeyboardInterrupt:
        log.info("Ctrl-C received, shutting down...")
