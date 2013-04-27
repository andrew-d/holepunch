#include <set>
#include <vector>
#include <string>
#include <iostream>
#include <stdint.h>

#include <boost/algorithm/string.hpp>
#include <tclap/CmdLine.h>

#include "cpplog.hpp"
#include "config.h"
#include "transports.hpp"

#define __STRINGIFY(x)  #x
#define STRINGIFY(x)    __STRINGIFY(x)

#define VERSION     STRINGIFY(holepunch_VERSION_MAJOR) "." \
                    STRINGIFY(holepunch_VERSION_MINOR) "." \
                    STRINGIFY(holepunch_VERSION_PATCH)


struct _options_t {
    bool                        client;
    std::string                 password;
    int                         verbosity;
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
    LOG_INFO(logger) << "Finished parsing arguments" << std::endl;
}


void RunClient() {

}


void RunServer() {
    // Launch threads.
    if( options.methods.find("tcp") != options.methods.end() ) {
        LOG_INFO(logger) << "Starting TCP transport..." << std::endl;
    }

    if( options.methods.find("udp") != options.methods.end() ) {
        LOG_INFO(logger) << "Starting UDP transport..." << std::endl;
    }

    if( options.methods.find("icmp") != options.methods.end() ) {
        LOG_INFO(logger) << "Starting ICMP transport..." << std::endl;
    }

    if( options.methods.find("dns") != options.methods.end() ) {
        LOG_INFO(logger) << "Starting DNS transport..." << std::endl;
    }
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
