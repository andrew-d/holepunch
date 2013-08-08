package holepunch

import (
    flag "github.com/ogier/pflag"
)

const MAJOR_VER = 1
const MINOR_VER = 0

// Message that the client sends to the server to see if the server is
// responding, and, if so, start the authentication procedure.
type ClientInitialRequest struct {
    // Client's hostname
    hostname string
}

// Message the server sends to the client to check version and start the
// authentication process.
type ServerInitialResponse struct {
    // Server version
    majorVer int
    minorVer int

    // Challenge for the client
    challenge string
}

// Message the client sends to the server to complete authentication.
type ClientChallengeResponse struct {
    // Response from the challenge
    challengeResp string
}

// Authentication result from the server.
type ServerAuthenticationResult struct {
    // Success or failure.
    authenticationSuccess bool
}

/* The protocol for communication is simple:
 *  - In the negotiation phase, the client sends messages back and forth
 *    to the server to verify connectivity, check versions, and authenticate.
 *          Client                 Server
 *      check_version   -->          *
 *            *         <--     server_version
 *                                   +
 *                               challenge
 *         response     -->          *
 *            *         <--        result
 *
 *    The messages can be distinguished by the first byte, as follows:
 *          0x00    data
 *          0x01    check_version
 *          0x02    server_challenge
 *          0x03    client_challenge_response
 *          0x04    server_auth_result
 *
 *    Note: if the challenge response is not received within 10 seconds, then
 *    the server will close the connection without sending a result.
 *
 *  - After authentication succeeds, all further messages are just binary blobs
 *    that contain packets to be forwarded.  Note that the underlying transport
 *    may impose some overhead (e.g. the TCP transport will prefix packets with
 *    the length, since TCP is a stream-oriented protocol, and UDP might need
 *    to include a header for reliable delivery).
 */

// Global options
var ipaddr string
var netmask string
var password string

func addCommonOptions(f *flag.FlagSet) {
    f.StringVar(&ipaddr, "ip", "", "the IP address of the TUN/TAP device")
    f.StringVar(&netmask, "netmask", "255.255.0.0", "the netmask of the TUN/TAP device")
    f.StringVar(&password, "pass", "insecure", "password for authentication")
}
