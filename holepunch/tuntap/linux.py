import os
import struct
import fcntl
from threading import Thread

import evergreen.queue

from .util import get_free_tun_interface


# Constants.
TUNSETIFF   = 0x400454ca
TUNSETOWNER = TUNSETIFF + 2     # TODO: accessibility?
IFF_TUN     = 0x000000001
IFF_TAP     = 0x000000002
IFF_NO_PI   = 0x00001000


def read_from_tuntap(fileno, q):
    while True:
        pkt = os.read(fileno, 65535)
        q.put(pkt, True)


class LinuxTunTapDevice(object):
    def __init__(self):
        # Find a free TUN device.
        self.name = get_free_tun_interface()

        # Open the TUN device file.
        self.dev = open('/dev/net/tun', 'r+b')

        # Create queue.
        self.queue = evergreen.queue.Queue(1)

        # Tell it what device we want.
        msg = struct.pack('16sH', self.name, IFF_TUN | IFF_NO_PI)

        # TODO: Check errors here.
        fcntl.ioctl(self.dev, TUNSETIFF, msg)

        self.thread = Thread(target=read_from_tuntap,
                             args=(self.dev.fileno(), self.queue))

    def setup(self):
        self.thread.daemon = True
        self.thread.start()

    def send_packet(self, packet):
        os.write(self.dev.fileno(), packet)

    def get_packet(self, timeout=None):
        if timeout is None:
            return self.queue.get(True)
        else:
            try:
                return self.queue.get(False, timeout)
            except evergreen.queue.Empty:
                return None


# This is the name we actually instantiate, for cross-platform compatibility.
TunTapDevice = LinuxTunTapDevice
