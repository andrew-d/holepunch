#pragma once

#ifndef _TRANSPORTS_H
#define _TRANSPORTS_H


class IPacketClient {
public:
    // Send a packet to the transport.  May block if necessary.
    virtual bool SendPacket(std::vector<uint8_t> pkt) = 0;

    // Get a packet from the transport.  Block indefinitely, or for a given
    // time in seconds.
    virtual bool GetPacket(std::vector<uint8_t>& out) = 0;
    virtual bool GetPacket(std::vector<uint8_t>& out, uint32_t timeout) = 0;

    // Helper function - get the name.
    virtual const char* Name() = 0;
};


class IPacketServer {
public:
    virtual void Start() = 0;
    virtual IPacketClient* AcceptClient() = 0;

    // Helper function - get the name.
    const char* Name();
};

#endif
