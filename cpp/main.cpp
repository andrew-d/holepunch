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

    // Remote host (only if we're a client).
    std::string                 remote_host;
} options;


cpplog::BaseLogger* logger = new cpplog::OstreamLogger(std::cout);


void printUsage(char* argv0, po::options_description& options,
                bool need_newline = false) {
    std::cerr << "Usage: " << argv0 << " <command> [options]" << std::endl;

    // Because boost::program_options is stupid and I'm OCD.
    if( need_newline )
        std::cerr << "\n";

    std::cerr << options << std::endl;
    std::cerr << "Valid subcommands:\n"
              << "  server      Run the holepunch server\n"
              << "  client      Run the holepunch client\n"
              << "\n"
              << "To get more information, run \"holepunch client --help\" or \n"
              << "\"holepunch server --help\"\n"
              << std::endl;
}


int parseArguments(int argc, char** argv) {
    // Make arguments vector.
    std::vector<std::string> arguments(argv + 1, argv + argc);

    // Global options.
    po::options_description globalOptions("Global options");
    globalOptions.add_options()
        ("help,h", "Print help messages")
        ("version,V", "Print program version");

    // Common operations (i.e. to both client and server).
    po::options_description commonOptions("Common options");
    commonOptions.add_options()
        ("password,p", po::value<std::string>()->required(),
            "Password to use for authentication")
        ("methods,m", po::value<std::string>()->default_value("tcp,udp,icmp,dns"),
            "Comma-seperated list of methods of connection to enable.  Defaults "
            "to all available methods.")
        ("verbose,v", "Be more verbose")
        ("quiet,q", "Be quieter - only show warnings or errors");

    // Client-specific options.
    po::positional_options_description clientPositionalOptions;
    clientPositionalOptions.add("remote_host", 1);

    // Server-specific options.
    // NOTE: None right now.

    po::variables_map vars;

    // Figure out what parser to run by comparing the first argument.  If we
    // don't have enough arguments, we print an error and then the usage.
    if( argc < 2 ) {
        std::cerr << "Error: no subcommand given!\n" << std::endl;
        printUsage(argv[0], globalOptions, true);
        return -1;
    }

    // Compare the first string.
    po::options_description allOptions;
    allOptions.add(globalOptions);

    bool validSubcommand = false;

    std::string cmd = arguments[0];
    arguments.erase(arguments.begin());

    try {

        if( "client" == cmd ) {
            // Use global, common, and client positional options.
            options.client = true;
            validSubcommand = true;
            po::store(po::command_line_parser(arguments)
                            .options(allOptions.add(commonOptions))
                            .positional(clientPositionalOptions).run(),
                      vars);

        } else if( "server" == cmd ) {
            // Use global and common options.
            options.client = false;
            validSubcommand = true;
            po::store(po::command_line_parser(arguments)
                            .options(allOptions.add(commonOptions)).run(),
                      vars);

        } else {
            // Do nothing (just generic options, above).
            po::store(po::command_line_parser(arguments)
                            .options(allOptions).run(),
                      vars);

        }

        if( vars.count("help") ) {
            printUsage(argv[0], allOptions);
            return -1;
        }

        po::notify(vars);

        // Parse verbosity/quiet.
        if( vars.count("quiet") ) {
            options.verbosity = LL_WARN;
        } else if( vars.count("verbose") ) {
            options.verbosity = LL_DEBUG;
        } else {
            options.verbosity = LL_INFO;
        }

        if( !validSubcommand ) {
            std::cerr << "Error: invalid subcommand \"" << cmd << "\"\n" << std::endl;
            printUsage(argv[0], allOptions);
            return -1;
        }

        // Split the methods string, by comma, into our set of methods.
        boost::split(options.methods,
                     vars["methods"].as<std::string>(),
                     boost::is_any_of(","));

        // If we're the client, we get the positional arguments now.
        if( options.client ) {

            if( vars.count("remote_host" ) > 0 ) {
                std::vector<std::string> positional =
                    vars["remote_host"].as< std::vector<std::string> >();
                std::cout << "\nLength of positional = " << positional.size() << std::endl;

                options.remote_host = positional[0];
            } else {
                std::cerr << "Error: no server name provided\n" << std::endl;
                printUsage(argv[0], allOptions);
                return -1;
            }
        }
    } catch( po::error& e ) {
        std::cerr << "Error: " << e.what() << "\n" << std::endl;
        printUsage(argv[0], allOptions);
        return -1;
    }

    // Set up proper logger now (since our first log statement is below).
    if( LL_DEBUG != options.verbosity ) {
        logger = new cpplog::FilteringLogger(options.verbosity, logger);
    }

    LOG_DEBUG(logger) << "Successfully parsed arguments" << std::endl;

    return 0;
}


void RunClient() {
    /* TCPPacketClient* t = new TCPPacketClient(options.host); */
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
    int ret = parseArguments(argc, argv);
    if( ret != 0 ) {
        if( ret < 0 ) {
            return 0;
        } else {
            return ret;
        }
    }

    if( options.client ) {
        LOG_INFO(logger) << "Running client..." << std::endl;
        RunClient();
    } else {
        LOG_INFO(logger) << "Running server..." << std::endl;
        RunServer();
    }
}
