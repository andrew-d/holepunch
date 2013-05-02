"""
Usage: holepunch client [options] <address>

Options:
    --methods METH      Methods to try
"""
import hmac
import hashlib
import logging
import threading


from . import transports
from .common import forward_packets


log = logging.getLogger(__name__)


# Architecture:
# -------------
# Unlike the server, the client is pretty simple :-)  We start two threads -
# one that will forward packets from the TUN device to the server, and then one
# to forward from the server to the TUN device.  Our main thread will then wait
# for these two threads to exit (see server.py for comment regarding signals).


def run(device, arguments):
    log.debug("Holepunching with server '%s'...", arguments['<address>'])

    # Try each method of connection.
    methods = arguments['--methods']
    if methods is None:
        methods = 'tcp,udp,icmp,dns'
    methods = [x.strip() for x in methods.split(',')]

    found = False
    conn = None
    for method in methods:
        log.info("Trying method %s...", method)
        mod = getattr(transports, method)

        # Try and create the transport.
        transport = mod.connect(arguments['<address>'])
        if not transport:
            continue

        # Test the transport.
        pwd = arguments['--password'] or ''
        if test_transport(transport, pwd):
            log.info("Transport '%s' successfully connected!", method)
            found = True
            conn = transport
            break

    if found is False:
        log.error("Did not find a transport that works!")
        return

    # Forward packets.
    t1 = threading.Thread(target=forward_packets,
                          name="server_to_tun",
                          args=(conn, device))
    t2 = threading.Thread(target=forward_packets,
                          name="tun_to_server",
                          args=(device, conn))

    # Start threads
    log.info("Starting forwarding threads...")
    t1.daemon = True
    t2.daemon = True
    t1.start()
    t2.start()

    # Wait for all threads.  Note that we have a timeout here so we don't stop
    # signals from being processed.
    log.info("Waiting for forwarding threads to finish...")
    threads = [t1, t2]

    try:
        while True:
            if len(threads) == 0:
                break

            # Wait for the thread.
            t = threads[0]
            t.join(2.0)

            # If it has exited, remove it from our list.
            if not t.is_alive():
                log.debug("Thread %s has exited", t.name)
                threads.remove(t)
    except KeyboardInterrupt:
        pass

    log.info("Client is finished")


def test_transport(transport, password):
    # Read the nonce from the transport.
    nonce = transport.get_packet()

    # Compute the HMAC of this challenge
    hm = hmac.new(password, digestmod=hashlib.sha256)
    hm.update(nonce)

    # Send the response back.
    transport.send_packet(hm.hexdigest())

    # Get a packet.
    ret = transport.get_packet()
    if ret == 'success':
        return True
    elif ret == 'failure':
        return False
    else:
        return False
