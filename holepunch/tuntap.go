package holepunch

import (
    "log"
    "os/exec"
    "strings"
    "time"

    "github.com/andrew-d/holepunch/tuntap"
)

func getTuntap(is_client bool) tuntap.Device {
    log.Println("Opening TUN/TAP device...")
    tuntap, err := tuntap.GetTuntapDevice()
    if err != nil {
        log.Fatal(err)
    }

    // Configure the device.
    log.Println("Configuring TUN/TAP device...")
    configureTuntap(is_client, tuntap.Name())

    // Start reading from the TUN/TAP device.
    tuntap.Start()
    return tuntap
}

func configureTuntap(is_client bool, devName string) {
    // Set default IP address, if needed.
    if len(ipaddr) == 0 {
        if is_client {
            ipaddr = "10.93.0.2"
        } else {
            ipaddr = "10.93.0.1"
        }
    }

    // Need to run: ifconfig tunX 10.0.0.1 10.0.0.1 netmask 255.255.255.0 up
    var cmd *exec.Cmd
    if is_client {
        cmd = exec.Command("/sbin/ifconfig", devName, ipaddr, server_addr, "netmask", netmask, "up")
    } else {
        cmd = exec.Command("/sbin/ifconfig", devName, ipaddr, ipaddr, "netmask", netmask, "up")
    }

    out, err := cmd.Output()
    if err != nil {
        log.Printf("Error running configuration command: %s\n", err)
    } else {
        log.Printf("Configured successfully (output: '%s')\n", out)
    }

    // TODO: we should repeatedly check until the interface is up, rather than
    // just waiting for a given length
    <-time.After(1 * time.Second)

    // Get output of ifconfig.
    stat := exec.Command("/sbin/ifconfig")
    out, err = stat.Output()
    if err != nil {
        log.Printf("Error running status command: %s\n", err)
    } else {
        lines := strings.Split(string(out), "\n")

        var out_lines []string
        found := false
        for _, l := range lines {
            // We start grabbing at the tun device, and stop when we hit a blank line,
            // which indicates another device.
            if strings.HasPrefix(l, devName) {
                found = true
            } else if found && l == "" {
                found = false
            }

            if found {
                out_lines = append(out_lines, l)
            }
        }

        log.Printf("ifconfig output:\n%s", strings.Join(out_lines, "\n"))
    }
}
