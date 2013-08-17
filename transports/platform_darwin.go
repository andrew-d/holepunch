// +build darwin
package transports

// This file contains Darwin-specific code.

import (
    //"fmt"
)

// TODO: error on unsupported platforms?
func ChangeRespondToPings(enabled bool) (bool, error) {
    return false, nil
}
