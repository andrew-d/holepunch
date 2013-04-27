#pragma once

#ifndef _TCP_H
#define _TCP_H

#include "transports.hpp"


class TCPPacketClient : public IPacketClient {
private:
    int     m_socket;

public:
    TCPPacketClient(std::string& host);
    TCPPacketClient(int socket)
        : m_socket(socket)
    { }
    virtual ~TCPPacketClient() {
        close(m_socket);
    }

    bool SendPacket(std::vector<uint8_t> pkt);
    bool GetPacket(std::vector<uint8_t>& out);
    bool GetPacket(std::vector<uint8_t>& out, uint32_t timeout);
    const char* Name();
};


class TCPPacketServer : public IPacketServer {

};


#endif
