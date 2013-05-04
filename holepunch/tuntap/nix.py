import os
import logging

import pyuv
import evergreen

from .util import get_free_tun_interface


log = logging.getLogger(__name__)


class GenericNixTunTapDevice(object):
    """
    This implements the common "read from a file descriptor" logic that is
    common to both Darwin and Linux.
    """
    def __init__(self):
        self.dev = self._init()
        self.queue = evergreen.queue.Queue(1)

    def _init(self):
        raise NotImplementedError("Must implement initialization in derived "
                                  "class")

    def setup(self):
        evergreen.tasks.spawn(self._start_read)

    def _start_read(self):
        loop = evergreen.loop.get_loop()._loop
        pipe = pyuv.Pipe(loop)
        pipe.open(self.dev)
        pipe.start_read(self._read_callback)

    def _read_callback(self, pipe, data, err):
        if err is not None:
            self.queue.put(data)
        else:
            self.queue.put(err)

    def send_packet(self, packet):
        os.write(self.dev, packet)

    def get_packet(self, timeout=None):
        # The timeout must be a float.
        if timeout is not None and not isinstance(timeout, float):
            timeout = float(timeout)

        # Read from our queue.
        try:
            return self.queue.get(True, timeout)
        except evergreen.queue.Empty:
            return None

    def close(self):
        os.close(self.dev)
        # TODO: shut down reading task?
