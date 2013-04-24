// +build darwin
package tuntap

import (
    "log"
    "os"
)

func GetTuntapDevice() (*os.File, string, error) {
    name, err := getTuntapName()
    if err != nil {
        log.Printf("Error getting name: %s\n", err)
        return nil, "", err
    }

    tuntapDev, err := os.OpenFile("/dev/"+name, os.O_RDWR, 0666)
    if err != nil {
        log.Printf("Error opening file: %s\n", err)
        return nil, "", err
    }

    return tuntapDev, name, nil
}
