package transports

// NOTE:
// UDP packets here can be spoofed, so it is necessary to have the encryption/
// authentication layer working too.  There's no point in using a sequence
// number or something similar, since we don't make any guarantees about the
// delivery of packets (similar to the internet as a whole).
