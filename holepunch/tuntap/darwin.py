import os
import select
import logging
from threading import Thread, Event

from .util import get_free_tun_interface


log = logging.getLogger(__name__)


class DarwinTunTapDevice(object):
    def __init__(self):
        # Find a free TUN device.
        self.name = get_free_tun_interface()

        # Open the device.
        log.debug("Opening device: %s", "/dev/" + self.name)
        self.dev = os.open('/dev/' + self.name, os.O_RDWR)

    def setup(self):
        pass

    def send_packet(self, packet):
        os.write(self.dev, packet)

    def get_packet(self, timeout=None):
        # The timeout must be a float.
        if timeout is not None and not isinstance(timeout, float):
            timeout = float(timeout)

        # We keep looping until we read something.
        try:
            rlist, wlist, xlist = select.select([self.dev], [], [], timeout)
        except select.error:
            return None

        if len(rlist) > 0 and rlist[0] == self.dev:
            packet = os.read(self.dev, 65535)
            return packet

        # If we get here, it's because we timed out.
        return None

    def close(self):
        os.close(self.dev)


# This is the name we actually instantiate, for cross-platform compatibility.
TunTapDevice = DarwinTunTapDevice
