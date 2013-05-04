import os
import fcntl
import struct

from .util import get_free_tun_interface
from .nix import GenericNixTunTapDevice


# Constants.
TUNSETIFF   = 0x400454ca
TUNSETOWNER = TUNSETIFF + 2     # TODO: accessibility?
IFF_TUN     = 0x000000001
IFF_TAP     = 0x000000002
IFF_NO_PI   = 0x00001000


class LinuxTunTapDevice(GenericNixTunTapDevice):
    def _init(self):
        # Find a free TUN device.
        self.name = get_free_tun_interface()

        # Open the TUN device file.
        fd = os.open('/dev/net/tun', os.O_RDWR)

        # Tell it what device we want.
        msg = struct.pack('16sH', self.name, IFF_TUN | IFF_NO_PI)

        # TODO: Check errors here.
        fcntl.ioctl(fd, TUNSETIFF, msg)

        return fd


# This is the name we actually instantiate, for cross-platform compatibility.
TunTapDevice = LinuxTunTapDevice
