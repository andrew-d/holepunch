package tuntap

import (
    "net"
    "fmt"
    "strconv"
)

func getTuntapName() (string, error) {
    // Get a list of interfaces.
    interfaces, err := net.Interfaces()
    if err != nil {
        return "", err
    }

    // Find all the ones that start with "tun".
    var largest int = -1
    for i := range interfaces {
        currName := interfaces[i].Name
        if len(currName) > 3 && currName[0:3] == "tun" {
            currNum, err := strconv.Atoi(currName[3:])
            if err == nil {
                if currNum > largest {
                    largest = currNum
                }
            }
        }
    }

    // Return 1 + the largest device we have.
    return fmt.Sprintf("tun%d", largest + 1), nil
}

