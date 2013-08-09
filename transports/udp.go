package transports

// NOTE:
// If we're going to include a UDP transport in holepunch, there needs to be
// some way of preventing spoofed UDP packets.  For example: HMAC'ing each
// packet with the shared secret we use for authentication (or some derived
// version of that).  Note that **CRYPTO IS HARD**, and there will probably
// still be bugs in this code - mention it prominently in the documentation.
