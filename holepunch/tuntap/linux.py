import os
import struct
import fcntl

from .util import get_free_tun_interface


# Constants.
TUNSETIFF   = 0x400454ca
TUNSETOWNER = TUNSETIFF + 2     # TODO: accessibility?
IFF_TUN     = 0x000000001
IFF_TAP     = 0x000000002
IFF_NO_PI   = 0x00001000


class LinuxTunTapDevice(object):
    def __init__(self):
        # Find a free TUN device.
        self.name = get_free_tun_interface()

        # Open the TUN device file.
        self.dev = open('/dev/net/tun', 'r+b')

        # Tell it what device we want.
        msg = struct.pack('16sH', self.name, IFF_TUN | IFF_NO_PI)

        # TODO: Check errors here.
        fcntl.ioctl(self.dev, TUNSETIFF, msg)

    def setup(self):
        pass

    def write_packet(self, packet):
        pass

    def get_packet(self):
        pass


# This is the name we actually instantiate, for cross-platform compatibility.
TunTapDevice = LinuxTunTapDevice
