# holepunch

## What is holepunch?

holepunch is a program that attempts to bypass internet filtering, firewalls, captive portals, and so on, by using various "transports" - i.e. methods of connection - to tunnel packets from a client to a server.  It's written in Google Go, and can be deployed as a single statically-linked binary.

## What operating systems does it work on?

Currently, it's being tested on Ubuntu 12.04 and Mac OS X.  I do plan on adding support for Windows clients in the future, but no attempt will be made to run the server on Windows.

## Is it secure?

**NO**.  I make every attempt to secure holepunch, including using a form of authentication based off of a shared secret, and encrypting all traffic, but this does *not* mean that it's secure.  Until it's been around for many years, and has been tested and audited by crypto security professionals (i.e. not me), you should assume that anything sent over holepunch to be no different than sending it in plaintext over the internet.

If you want security, use something like OpenVPN or Tor.

## What can it do?

Currently, it supports tunneling traffic over TCP.  I will be adding UDP, ICMP, and DNS-based transports (in that order) before it's considered "stable".
