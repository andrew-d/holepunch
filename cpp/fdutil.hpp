#pragma once

#ifndef _FDUTIL_H
#define _FDUTIL_H

ssize_t readn(int fd, void *vptr, size_t n);
ssize_t writen(int fd, const void *vptr, size_t n);


#endif
