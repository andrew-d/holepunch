#include <string>
#include <vector>

#include <errno.h>
#include <sys/types.h>
#include <sys/socket.h>
#include <netinet/in.h>
#include <netdb.h>
#include <stdint.h>

#include "tcp.hpp"
#include "cpplog.hpp"
#include "fdutil.hpp"

#define PORT    44460

extern cpplog::OstreamLogger logger;


typedef struct _packed_len_t {
    union {
        uint16_t length;
        uint8_t  bytes[2];
    };
} packed_len_t;


TCPPacketClient::TCPPacketClient(std::string& host) {
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


bool TCPPacketClient::SendPacket(std::vector<uint8_t> pkt) {
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

bool TCPPacketClient::GetPacket(std::vector<uint8_t>& out, uint32_t timeout) {
    return false;
}

bool TCPPacketClient::GetPacket(std::vector<uint8_t>& out) {
    // Read our packet length.
    packed_len_t len;
    ssize_t ret = readn(m_socket, &len, 2);
    if( ret < 0 ) {
        return false;
    }
    uint16_t length = ntohs(len.length);

    // Read this many bytes
    out.resize(length);
    ret = readn(m_socket, out.data(), length);
    return (ret >= 0);
}

const char* TCPPacketClient::Name() {
    return "TCP";
}



TCPPacketServer::TCPPacketServer() {
    started  = false;
    m_socket = socket(AF_INET, SOCK_STREAM, 0);
    if( 0 == m_socket ) {
        LOG_ERROR(logger) << "Error creating socket, errno = " << errno
            << std::endl;
        throw "Error creating socket";
    }
}

void TCPPacketServer::Start() {
    // Build the server's internet address.
    struct sockaddr_in servaddr;
    memset(&servaddr, 0, sizeof(servaddr));
    servaddr.sin_family      = AF_INET;
    servaddr.sin_addr.s_addr = htonl(INADDR_ANY);
    servaddr.sin_port        = htons(PORT);

    // Bind socket.
    int ret = bind(m_socket, (struct sockaddr *)&servaddr, sizeof(servaddr));
    if( ret < 0 ) {
        LOG_ERROR(logger) << "Error binding to port " << PORT << ": error "
            << ret << std::endl;
        return;
    }

    // Start listening.
    ret = listen(m_socket, 1);
    if( ret < 0 ) {
        LOG_ERROR(logger) << "Error calling listen(): " << ret << std::endl;
        return;
    }

    // Done!
    started = true;
}

IPacketClient* TCPPacketServer::AcceptClient() {
    // Accept and return a single client.
    int client_sock = 0;
    if ( (client_sock = accept(m_socket, NULL, NULL) ) < 0 ) {
        LOG_ERROR(logger) << "Error calling accept(): " << client_sock
            << std::endl;
        return NULL;
    }

    return new TCPPacketClient(client_sock);
}

const char* TCPPacketServer::Name() {
    return "TCP";
}
