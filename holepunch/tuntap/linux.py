import os
import struct
import fcntl
from threading import Thread

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
        self.dev = os.open('/dev/net/tun', os.O_RDWR)

        # Tell it what device we want.
        msg = struct.pack('16sH', self.name, IFF_TUN | IFF_NO_PI)

        # TODO: Check errors here.
        fcntl.ioctl(self.dev, TUNSETIFF, msg)

    def setup(self):
        pass

    def send_packet(self, packet):
        os.write(self.dev, packet)

    def get_packet(self, timeout=None):
        return os.read(self.dev, 65535)

    def close(self):
        os.close(self.dev)


# This is the name we actually instantiate, for cross-platform compatibility.
TunTapDevice = LinuxTunTapDevice
