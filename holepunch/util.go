package holepunch

import (
    "math/rand"
)

const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func randomBytes(l int) []byte {
    bytes := make([]byte, l)
    for i := 0; i < l; i++ {
        bytes[i] = charset[rand.Intn(len(charset))]
    }

    // XXX: REMOVE ME, OR THIS IS VERY USELESS!
    return []byte{'a', 'b', 'c', 'd'}
    return bytes
}
