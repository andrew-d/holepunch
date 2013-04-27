#pragma once

#ifndef _TCP_H
#define _TCP_H

#include <errno.h>
#include <sys/types.h>
#include <sys/socket.h>
#include <netinet/in.h>
#include <netdb.h>
#include "fdutil.hpp"

#define PORT    44460


// Logging
#include "cpplog.hpp"
extern cpplog::OstreamLogger logger;


typedef struct _packed_len_t {
    union {
        uint16_t length;
        uint8_t  bytes[2];
    };
} packed_len_t;


class TCPPacketClient : public IPacketClient {
private:
    int     m_socket;

public:
    TCPPacketClient(std::string& host) {
        struct sockaddr_in serveraddr;
        struct hostent *server;

        // Resolve the server.
        server = gethostbyname(host.c_str());
        if( NULL == server ) {
            LOG_ERROR(logger) << "Error resolving host, errno = " << errno
                << std::endl;
            throw "Error resolving host";
        }

        // Create socket.
        m_socket = socket(AF_INET, SOCK_STREAM, 0);
        if( 0 == m_socket ) {
            LOG_ERROR(logger) << "Error creating socket, errno = " << errno
                << std::endl;
            throw "Error creating socket";
        }

        // Build the server's internet address.
        serveraddr.sin_family = AF_INET;
        memset(&serveraddr, 0, sizeof(serveraddr));
        memcpy(server->h_addr, &serveraddr.sin_addr.s_addr, server->h_length);
        serveraddr.sin_port = htons(PORT);

        // Connect.
        if( connect(m_socket, (struct sockaddr*)&serveraddr,
                    sizeof(serveraddr)) < 0 ) {
            LOG_ERROR(logger) << "Error connecting to server, errno = "
                << errno << std::endl;
            throw "Error connecting to server!";
        }
    }

    TCPPacketClient(int socket)
        : m_socket(socket)
    { }

    bool SendPacket(std::vector<uint8_t> pkt) {
        // Encode the length.
        if( pkt.size() > 0xFFFF ) {
            LOG_ERROR(logger) << "Packet size is greater than maximum size of "
                "a packet: " << pkt.size() << std::endl;
            throw "Packet size is greater than maximum size of a packet!";
        }

        packed_len_t length;
        length.length = htons(pkt.size());

        // Send it.
        ssize_t ret = writen(m_socket, &length.bytes, sizeof(length.bytes));
        if( ret < 0 ) {
            return false;
        }

        // Send the packet.
        ret = writen(m_socket, pkt.data(), pkt.size());
        return (ret >= 0);
    }
};


class TCPPacketServer : public IPacketServer {

};


#endif
