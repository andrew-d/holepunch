import sys

if sys.platform == 'darwin':
    from .darwin import TunTapDevice
elif sys.platform.startswith('linux'):
    from .linux import TunTapDevice
else:
    raise RuntimeError("Don't have a TUN/TAP interface for this platform!")
