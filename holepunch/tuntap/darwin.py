import os
import select
import logging
from threading import Thread

import evergreen
import evergreen.tasks
import evergreen.queue

from .util import get_free_tun_interface


log = logging.getLogger(__name__)


# Apparently this must be run in a thread, using the real select() call,, or it
# does strange things.  Whatever - do it anyway.
def read_from_tuntap(fileno, q):
    while True:
        try:
            rlist, wlist, xlist = select.select([fileno], [], [])
        except select.error:
            continue

        if len(rlist) > 0 and rlist[0] == fileno:
            packet = os.read(fileno, 65535)

            # Put on the queue.  Note that we DO want to block here - though
            # only this current task will block.
            q.put(packet, True)


class DarwinTunTapDevice(object):
    def __init__(self):
        # Find a free TUN device.
        self.name = get_free_tun_interface()

        # Open the device.
        log.debug("Opening device: %s", "/dev/" + self.name)
        self.dev = os.open('/dev/' + self.name, os.O_RDWR)

        # Our queue is of size 1, as we want the thread (above) to block when
        # the queue already has a packet.
        self.queue = evergreen.queue.Queue(1)
        self.thread = Thread(target=read_from_tuntap,
                             args=(self.dev, self.queue))

    def setup(self):
        """Call this once the TUN device is configured, to start reading."""
        self.thread.daemon = True
        self.thread.start()

    def send_packet(self, packet):
        self.dev.write(packet)

    def get_packet(self, timeout=None):
        if timeout is None:
            return self.queue.get(True)
        else:
            try:
                return self.queue.get(False, timeout)
            except evergreen.queue.Empty:
                return None

    def close(self):
        os.close(self.dev)


# This is the name we actually instantiate, for cross-platform compatibility.
TunTapDevice = DarwinTunTapDevice
