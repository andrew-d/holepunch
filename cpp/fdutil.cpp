#include <unistd.h>
#include <errno.h>
#include <sys/types.h>
#include <sys/socket.h>
#include <netinet/in.h>
#include <stdint.h>


// Read n bytes from the given file descriptor, handling EINTR and reads that
// aren't fully complete (e.g. for a socket).
ssize_t readn(int fd, void *vptr, size_t n) {
    size_t   nleft;
    ssize_t  nread;
    uint8_t* ptr;

    ptr = static_cast<uint8_t*>(vptr);
    nleft = n;
    while( nleft > 0 ) {
        if( (nread = read(fd, ptr, nleft)) < 0 ) {
            // Continue on EINTR, but error on everything else.
            if (errno == EINTR) {
                nread = 0;
            } else {
                return (-1);
            }
        } else if ( nread == 0 ) {
            break;
        }

        nleft -= nread;
        ptr += nread;
    }

    return (n - nleft);
}


// Write all bytes to a given file descriptor.
ssize_t writen(int fd, const void *vptr, size_t n) {
    size_t nleft;
    ssize_t nwritten;
    const uint8_t *ptr;

    ptr = static_cast<const uint8_t*>(vptr);
    nleft = n;
    while( nleft > 0 ) {
        if( (nwritten = write(fd, ptr, nleft)) <= 0 ) {
            // Continue on EINTR, but error on everything else.
            if (nwritten < 0 && errno == EINTR) {
                nwritten = 0;
            } else {
                return (-1);
            }
         }

         nleft -= nwritten;
         ptr += nwritten;
    }

    return n;
}
