#pragma once

#ifndef _TRANSPORTS_H
#define _TRANSPORTS_H


class IPacketClient {
public:
    // Send a packet to the transport.  May block if necessary.
    virtual bool SendPacket(std::vector<uint8_t> pkt) = 0;

    // Get a packet from the transport.  Block indefinitely, or for a given
    // time in seconds.
    virtual std::vector<uint8_t> GetPacket() = 0;
    virtual std::vector<uint8_t> GetPacket(uint32_t timeout) = 0;

    // Helper function - get the name.
    virtual const char* Name() = 0;
};


class IPacketServer {
public:
    virtual void Start() = 0;
    virtual IPacketClient* AcceptClient() = 0;
};


// Include actual transports here.
#include "tcp.hpp"
#include "udp.hpp"


#endif
