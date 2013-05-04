import os
import logging

import evergreen.queue

from .util import get_free_tun_interface
from .nix import GenericNixTunTapDevice


log = logging.getLogger(__name__)


class DarwinTunTapDevice(GenericNixTunTapDevice):
    def _init(self):
        # Find a free TUN device.
        self.name = get_free_tun_interface()

        # Open the device.  TODO: check for errors.
        log.debug("Opening device: %s", "/dev/" + self.name)
        fd =  os.open('/dev/' + self.name, os.O_RDWR)

        return fd

# This is the name we actually instantiate, for cross-platform compatibility.
TunTapDevice = DarwinTunTapDevice
