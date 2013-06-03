package transports

/* Reliable transport
 * ------------------
 * In order for non-TCP transports to work, we need to implement some sort of
 * reliability layer on top of them.  We do this by implementing a pseudo-
 * transport that takes another transport as an input, and uses a simple
 * algorithm to make it reliable.  Details of this algorithm are below:
 *
 * TODO: Implement basic go-back-n algorithm (or the like), and then maybe
 *       investigate implementing something like TCP {Reno,Nevada,Los Vegas}
 *
 */

