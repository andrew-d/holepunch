import select
import socket
import logging

from .transports.base import SocketDisconnected


log = logging.getLogger(__name__)


def forward_packets(src, dst, srcname=None, dstname=None):
    try:
        while True:
            pkt = src.get_packet()
            if pkt is None:
                log.error("pkt is none (src = %r, dst = %r)", src, dst)
                continue

            dst.send_packet(pkt)

            if srcname is not None and dstname is not None:
                log.info('%s --> %s (%d bytes)', srcname, dstname, len(pkt))
    except (socket.error, select.error, SocketDisconnected):
        log.exception("Socket or select error, stopping forwarding loop...")


def wait_for_multiple_tasks(tasks):
    # We want to iterate over every item once each iteration, since it's
    # possible that the first task that we check never finishes, but we
    # want to ensure that a transport that finishes is marked as such as
    # soon as possible.  We also can't modify a list while iterating over
    # it.
    waiters = list(tasks)
    while True:
        if len(waiters) == 0:
            break

        # Keep track of the finished tasks, so we can remove them once we
        # finish iterating over the list.
        removed = []

        for t in waiters:
            finished = t.join(2.0)

            # If it has exited, remove it from our list.
            if finished:
                log.debug("Task %s has exited", t.name)
                removed.append(t)

        for t in removed:
            waiters.remove(t)
