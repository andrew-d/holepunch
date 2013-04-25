from abc import ABCMeta, abstractmethod, abstractproperty


class ConnectionError(Exception):
    pass


class ClientBase(object):
    __metaclass__ = ABCMeta

    @abstractmethod
    def get_packet(self, timeout=None):
        pass

    @abstractmethod
    def send_packet(self, packet_data):
        pass

    @abstractproperty
    def name(self):
        pass


class ServerBase(object):
    __metaclass__ = ABCMeta

    pass
