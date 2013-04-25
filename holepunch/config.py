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
