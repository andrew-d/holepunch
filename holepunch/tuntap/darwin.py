import os

import evergreen
import evergreen.tasks
import evergreen.channel
from evergreen.lib import select

from .util import get_free_tun_interface


def read_from_tuntap(fileno, channel):
    while True:
        rlist, wlist, xlist = select.select([fileno], [], [])
        if len(rlist) > 0:
            packet = os.read(fileno, 65535)
            channel.send(packet)


class DarwinTunTapDevice(object):
    def __init__(self):
        # Find a free TUN device.
        self.name = get_free_tun_interface()

        # Open the device.
        self.dev = os.open('/dev/' + self.name, os.O_RDWR)
        self.chan = evergreen.channel.Channel()

    def setup(self):
        """Call this once the TUN device is configured, to start reading."""
        evergreen.tasks.spawn(read_from_tuntap, self.dev, self.chan)

    def write_packet(self, packet):
        self.dev.write(packet)

    def get_packet(self):
        return self.chan.receive()

    def close(self):
        os.close(self.dev)


# This is the name we actually instantiate, for cross-platform compatibility.
TunTapDevice = DarwinTunTapDevice
