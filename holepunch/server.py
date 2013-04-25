"""
Usage: holepunch server [options]

Options:
    --methods METH      Methods to start (comma-seperated list of: tcp, udp,
                        icmp, dns).  Defaults to all of the methods.
"""
def run(device, arguments):
    methods = arguments['--methods']
    if methods is None:
        methods = 'tcp,udp,icmp,dns'
    methods = methods.split(',')
