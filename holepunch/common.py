import logging


log = logging.getLogger(__name__)


def forward_packets(src, dst, srcname=None, dstname=None):
    while True:
        pkt = src.get_packet()
        if pkt is None:
            log.error("pkt is none (src = %r, dst = %r)", src, dst)
            continue

        dst.send_packet(pkt)

        if srcname is not None and dstname is not None:
            log.info('%s --> %s (%d bytes)', srcname, dstname, len(pkt))
