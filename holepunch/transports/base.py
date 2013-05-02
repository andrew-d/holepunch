from abc import ABCMeta, abstractmethod, abstractproperty

from ..six import with_metaclass


class ConnectionError(Exception):
    pass


class SocketDisconnected(Exception):
    pass


class ClientBase(with_metaclass(ABCMeta)):
    @abstractmethod
    def get_packet(self, timeout=None):
        raise NotImplementedError

    @abstractmethod
    def send_packet(self, packet_data):
        raise NotImplementedError

    @abstractmethod
    def close(self):
        raise NotImplementedError

    @abstractproperty
    def name(self):
        raise NotImplementedError


class ServerBase(with_metaclass(ABCMeta)):
    @abstractmethod
    def accept_client(self):
        raise NotImplementedError

    @abstractproperty
    def name(self):
        raise NotImplementedError
