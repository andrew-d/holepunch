// +build darwin
package transports

// This file contains Darwin-specific code.

import (
    //"fmt"
)

// TODO: error on unsupported platforms?
func ChangeIgnorePings(enabled bool) (bool, error) {
    return false, nil
}
