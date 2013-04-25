"""Simple, low-configuration VPN

Usage:
    holepunch client [--netmask=MASK] [--methods=METHODS] <address>
    holepunch server [--netmask=MASK]
    holepunch --version
    holepunch (-h | --help)

Options:
    -h --help       Show this screen.
    --version       Show version.

"""
import logging

from docopt import docopt

from .config import set_interface_properties
from .tuntap import TunTapDevice
from .client import client
from .server import server
from .log import setup_logging


log = logging.getLogger(__name__)


def main():
    setup_logging()
    arguments = docopt(__doc__, version='Holepunch v0.0.1')

    # Set up TUN device - we need this for both server and client.
    log.info("Creating TUN device...")
    dev = TunTapDevice()

    # Configure the device.
    log.info("Configuring the TUN device...")
    if arguments['client']:
        set_interface_properties(dev.name, '10.93.0.2', '10.93.0.1')
    else:
        set_interface_properties(dev.name, '10.93.0.1', '10.93.0.1')

    # Start the device now that we've configured it.
    dev.setup()

    if arguments['client']:
        client(dev, arguments)
    else:
        server(dev, arguments)


if __name__ == "__main__":
    main()
