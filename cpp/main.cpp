#include <set>
#include <vector>
#include <string>
#include <iostream>
#include <stdint.h>

#include <boost/algorithm/string.hpp>
#include <boost/assign/std/vector.hpp>
#include <boost/date_time.hpp>
#include <boost/thread/thread.hpp>
#include <boost/program_options.hpp>
#include <tclap/CmdLine.h>

#include "cpplog.hpp"
#include "config.h"
#include "transports.hpp"

#include "tcp.hpp"

namespace po = boost::program_options;


#define __STRINGIFY(x)  #x
#define STRINGIFY(x)    __STRINGIFY(x)

#define VERSION     STRINGIFY(holepunch_VERSION_MAJOR) "." \
                    STRINGIFY(holepunch_VERSION_MINOR) "." \
                    STRINGIFY(holepunch_VERSION_PATCH)


/* Architecture:
 * ===============
 * Server:
 * -------
 * We start a thread for each transport method.  The transports will block on
 * accepting clients, and return a new IPacketClient for each new client.  When
 * we get a new client, then we start a new thread.  This thread will read from
 * the client's connection, and write to the TUN device (directly, since writes
 * from multiple threads are safe to perform, so long as all writes are
 * only performed as a single syscall).  The new clients are also added to a
 * single list of connected clients.
 * We also start a single thread that will read packets from the TUN device as
 * they arrive.  When a packet arrives, this thread will traverse the list of
 * all clients and send the packet to all of them.  Note that this list must
 * be protected with a lock!
 *
 * Client:
 * -------
 * We try to connect to the server with each method in order.  When one
 * succeeds, we start a new thread that will forward from the TUN device to our
 * transport, and then call (from the main thread) a function that will forward
 * from our transport to the TUN device.
 */


struct _options_t {
    // Whether we're a client (or, if false, a server)
    bool                        client;

    // Password to use for authentication.
    std::string                 password;

    // Verbosity level.
    int                         verbosity;

    // Connection methods to use (e.g. TCP, UDP, etc.)
    std::set<std::string>       methods;
} options;


cpplog::OstreamLogger logger(std::cout);


void parseArguments(int argc, char** argv) {
    try {
        TCLAP::CmdLine cmd("Some description here", ' ', VERSION);

        std::vector<std::string> allowed;
        allowed.push_back("client");
        allowed.push_back("server");
        TCLAP::ValuesConstraint<std::string> allowedVals( allowed );

        TCLAP::UnlabeledValueArg<std::string> cmdArg("command",
                "Whether to run client or server", true, "", &allowedVals);
        cmd.add( cmdArg );

        TCLAP::SwitchArg verboseSwitch("v", "verbose",
                "Be verbose in outputing messages", cmd, false);
        TCLAP::SwitchArg quietSwitch("q", "quiet",
                "Only output warnings and errors", cmd, false);

        TCLAP::ValueArg<std::string> passwordArg("p", "password",
                "The password to use for authentication", true, "", "string");
        cmd.add(passwordArg);

        TCLAP::ValueArg<std::string> methodsArg("m", "methods",
                "A comma-seperated list of methods to use.  Valid methods "
                "are: tcp, udp, icmp, and dns.  If not given, will default "
                "to all methods", false, "tcp,udp,icmp,dns", "string");
        cmd.add(methodsArg);

        // Parse arguments here.
        cmd.parse( argc, argv );

        // Fill in options.
        options.password = passwordArg.getValue();
        options.client = (cmdArg.getValue() == "client");

        // Split the methods string, by comma, into our set of methods.
        boost::split(options.methods, methodsArg.getValue(),
                boost::is_any_of(","));
    } catch( TCLAP::ArgException &e) {
        LOG_ERROR(logger) << "Error: " << e.error() << " for arg " << e.argId() << std::endl;
    }

    LOG_DEBUG(logger) << "Finished parsing arguments" << std::endl;
}


void RunClient() {
    TCPPacketClient* t = new TCPPacketClient(options.host);
}


void StartTransport(IPacketServer* packetServer) {
    if( NULL == packetServer ) {
        return;
    }

    LOG_DEBUG(logger) << "In transport thread for transport "
        << packetServer->Name() << std::endl;

    LOG_INFO(logger) << "Starting transport..." << std::endl;
    packetServer->Start();

    while( 1 ) {
        IPacketClient* client = packetServer->AcceptClient();

        LOG_INFO(logger) << "Accepted new client on transport "
            << packetServer->Name() << std::endl;

        // TODO: do things with this client.

        // TODO: delete client
    }
}


void RunServer() {
    boost::thread_group serverThreads;

    if( options.methods.find("tcp") != options.methods.end() ) {
        LOG_INFO(logger) << "Starting TCP transport..." << std::endl;

        boost::thread* tcpThread = new boost::thread(StartTransport,
                (IPacketServer*)NULL);
        serverThreads.add_thread(tcpThread);
    }

    if( options.methods.find("udp") != options.methods.end() ) {
        LOG_INFO(logger) << "Starting UDP transport..." << std::endl;

        boost::thread* udpThread = new boost::thread(StartTransport,
                (IPacketServer*)NULL);
        serverThreads.add_thread(udpThread);
    }

    if( options.methods.find("icmp") != options.methods.end() ) {
        LOG_INFO(logger) << "Starting ICMP transport..." << std::endl;

        boost::thread* icmpThread = new boost::thread(StartTransport,
                (IPacketServer*)NULL);
        serverThreads.add_thread(icmpThread);
    }

    if( options.methods.find("dns") != options.methods.end() ) {
        LOG_INFO(logger) << "Starting DNS transport..." << std::endl;

        boost::thread* dnsThread = new boost::thread(StartTransport,
                (IPacketServer*)NULL);
        serverThreads.add_thread(dnsThread);
    }

    LOG_INFO(logger) << "Waiting for all threads to finish..." << std::endl;
    serverThreads.join_all();
    LOG_INFO(logger) << "All threads are done. Exiting server..." << std::endl;
}


int main(int argc, char** argv) {
    parseArguments(argc, argv);

    if( options.client ) {
        LOG_INFO(logger) << "Running client..." << std::endl;
        RunClient();
    } else {
        LOG_INFO(logger) << "Running server..." << std::endl;
        RunServer();
    }
}
