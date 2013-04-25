from ..interfaces import get_network_interfaces

def get_free_tun_interface():
    highest_num = -1
    for i in get_network_interfaces():
        name = i.name
        if name.startswith('tun'):
            try:
                curr = int(name[3:])
                if curr > highest_num:
                    highest_num = curr
            except ValueError:
                pass

    return "tun%d" % (highest_num + 1,)
