"""
This module is responsible for configuring the network - e.g. setting up an
interface, setting routes, and so on.
"""
import sys
import logging
import subprocess


log = logging.getLogger(__name__)


def set_interface_properties(interface, address, gateway, netmask=None):
    args = ['/sbin/ifconfig', interface, address, gateway]
    if netmask is not None:
        args.extend(['netmask', netmask])
    args.append('up')

    try:
        subprocess.check_output(args, stderr=subprocess.STDOUT)
    except subprocess.CalledProcessError as e:
        log.error("Could not configure interface:\nCommand: %s\nOutput: %s",
                  e.cmd, e.output)
        return False

    log.info("Successfully configured interface")
    return True


def set_route_gateway(interface, addr_spec, gateway):
    log.debug("Setting a route for interface %s: %s uses gateway %s",
              interface, addr_spec, gateway)

    if sys.platform.startswith('darwin'):
        args = ['/sbin/route', 'add', '-net', addr_spec, gateway]
    elif sys.platform.startswith('linux'):
        args = ['/sbin/ip', 'route', 'add', addr_spec, 'gw', gateway,
                interface]

    try:
        subprocess.check_output(args, stderr=subprocess.STDOUT)
    except subprocess.CalledProcessError as e:
        log.error("Could not set route:\nCommand: %s\nOutput: %s",
                  e.cmd, e.output)
        return False

    log.info("Successfully set route")
    return True


def get_gateway_for_ip(ip_addr):
    if sys.platform.startswith('darwin'):
        args = ['/sbin/route', 'get', ip_addr]
    elif sys.platform.startswith('linux'):
        args = ['/sbin/ip', 'route', 'get', ip_addr]

    try:
        out = subprocess.check_output(args, stderr=subprocess.STDOUT)
    except subprocess.CalledProcessError as e:
        log.error("Could not get gateway:\nCommand: %s\nOutput: %s",
                  e.cmd, e.output)
        return None

    if sys.platform.startswith('darwin'):
        # Format:
        #     $ route get 1.2.3.4
        #        route to: 1.2.3.4
        #     destination: 1.2.3.0
        #            mask: 255.255.255.0
        #         gateway: 10.93.0.1
        #       interface: en3
        #           flags: <UP,GATEWAY,DONE,STATIC,PRCLONING>
        #          recvpipe  sendpipe  ssthresh  rtt,msec    rttvar  hopcount      mtu     expire
        #            0         0         0         0         0         0      1500         0

        lines = [x.strip() for x in out.split('\n')]
        for line in lines:
            if line.startswith('gateway'):
                spl = line.split(':')
                if len(spl) >= 2:
                    return line.split(':')[1].strip()
    elif sys.platform.startswith('linux'):
        # Format:
        #       1.2.3.4 via 10.0.2.2 dev eth0  src 10.0.2.15
        #           cache

        lines = out.split('\n')
        if len(lines) >= 1:
            segs = lines[0].split(' ')
            if len(segs) >= 3:
                return segs[2]

    # Not parsed correctly, so we error out :-(
    log.error("Could not get default gateway for server.  Command "
              "output:\n" + out)
    return None
