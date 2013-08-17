// +build linux
package transports

// This file contains Linux-specific code.

import (
    "io/ioutil"
    "bytes"
)

func ChangeIgnorePings(ignore bool) (bool, error) {
    old_val, err := ioutil.ReadFile("/proc/sys/net/ipv4/icmp_echo_ignore_all")
    if err != nil {
        return false, err
    }

    // The old value (as a boolean) is whether or not the value is equal to "1".
    old_bool := bytes.Equal(old_val, []byte("1"))

    // If we are to ignore, then we write a "1", otherwise, a "0".
    var new_val []byte
    if ignore {
        new_val = []byte("1")
    } else {
        new_val = []byte("0")
    }

    err = ioutil.WriteFile("/proc/sys/net/ipv4/icmp_echo_ignore_all",
        new_val, 0644)
    if err != nil {
        return false, err
    }

    return old_bool, nil
}
