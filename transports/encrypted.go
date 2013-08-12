package transports

import (
    "crypto/cipher"
    "crypto/aes"
    "crypto/hmac"
    "crypto/subtle"
    "crypto/rand"
    "crypto/sha256"
    "fmt"
    "io"
    "log"
)

// This package implements a simple encrypted transport on top of an existing
// transport.  In general, it runs AES in CTR mode over all the data that's
// being sent and received.  The encryption key is generated from a shared that
// must be provided.  Note that this doesn't (necessarily) need to be the same
// as the authentication secret, just that it must be equal on both side of the
// connection.

type EncryptedPacketClient struct {
    underlying PacketClient
    our_stream cipher.Stream
    other_stream cipher.Stream
}

const TEST_STRING = "this is a test string"

func NewEncryptedPacketClient(underlying PacketClient, secret string) (*EncryptedPacketClient, error) {
    // Current plans:
    //  - Each side sends a random IV, MAC'd with the shared secret, then
    //    sets up the stream cipher using the IVs and the shared secret as
    //    the key.  Consider using PBKDF2 for the key (since just hashing is
    //    insufficient security - though do we assume it must be hashed
    //    already?)
    //  - Each side then encrypts and sends a test string with the current
    //    setup (stream cipher, IV, etc.)
    //  - On receipt of the other side's message, it is decrypted and
    //    verified to match the test string.
    //
    // NOTES:
    //  - Crypto is hard
    //  - This will probably^H^H^H for sure be buggy - need to make sure people
    //    know this
    //  - Encrypt, then MAC
    //  - Random IVs for each initialization, don't re-use IVs or nonces (CTR)
    //  - Constant-time comparison
    //
    // TODO:
    //  - Close underlying client on error?
    secret_bytes := []byte(secret)

    our_iv := make([]byte, aes.BlockSize)
    n, err := io.ReadFull(rand.Reader, our_iv)
    if err != nil {
        return nil, err
    } else if n != len(our_iv) {
        return nil, fmt.Errorf("read less than required bytes (%d < %d)",
            n, len(our_iv))
    }

    hm := hmac.New(sha256.New, secret_bytes)
    _, err = hm.Write(our_iv)
    if err != nil {
        return nil, err
    }
    our_iv_mac := hm.Sum(nil)

    // We have our IV, and the authenticated version of it.  We launch a
    // goroutine to concurrently send our IV to the other end of the
    // connection, and then read the other side's IV.
    go func() {
        pkt := make([]byte, len(our_iv) + len(our_iv_mac))
        copy(pkt, our_iv)
        copy(pkt[len(our_iv):], our_iv_mac)
        underlying.SendChannel() <- pkt
    }()

    // Read from the other side.
    other_pkt := <-underlying.RecvChannel()
    other_iv := other_pkt[0:aes.BlockSize]
    other_iv_mac := other_pkt[aes.BlockSize:]

    // Reset our HMAC
    hm = hmac.New(sha256.New, secret_bytes)
    _, err = hm.Write(other_iv)
    if err != nil {
        return nil, err
    }

    if subtle.ConstantTimeCompare(other_iv_mac, hm.Sum(nil)) != 1 {
        return nil, fmt.Errorf("remote IV hmac is invalid")
    }

    // Use these values to set up two stream ciphers.
    our_block, err := aes.NewCipher(secret_bytes)
    if err != nil {
        return nil, err
    }
    our_stream := cipher.NewCTR(our_block, our_iv)

    other_block, err := aes.NewCipher(secret_bytes)
    if err != nil {
        return nil, err
    }
    other_stream := cipher.NewCTR(other_block, other_iv)

    // Encrypt the test string with each stream, and then send/receive.
    our_enc := make([]byte, len(TEST_STRING))
    other_enc := make([]byte, len(TEST_STRING))
    our_stream.XORKeyStream(our_enc, []byte(TEST_STRING))
    other_stream.XORKeyStream(other_enc, []byte(TEST_STRING))

    go func() {
        underlying.SendChannel() <- our_enc
    }()

    other_data := <-underlying.RecvChannel()

    if subtle.ConstantTimeCompare(other_enc, other_data) != 1 {
        return nil, fmt.Errorf("remote encrypted test string doesn't match")
    }

    // If we get here, it all works!
    ret := &EncryptedPacketClient{underlying, our_stream, other_stream}
    return ret, nil
}

func (c *EncryptedPacketClient) SendChannel() chan []byte {
    return c.underlying.SendChannel()
}

func (c *EncryptedPacketClient) RecvChannel() chan []byte {
    return c.underlying.RecvChannel()
}

func (c *EncryptedPacketClient) Close() {
    c.underlying.Close()
}

func (c *EncryptedPacketClient) IsReliable() bool {
    return c.underlying.IsReliable()
}

func (c *EncryptedPacketClient) Describe() string {
    return "EncryptedPacketClient"
}

type EncryptedTransport struct {
    accept_ch  chan PacketClient
    underlying chan PacketClient
    secret     string
}

func (t *EncryptedTransport) AcceptChannel() chan PacketClient {
    return t.accept_ch
}

func (t *EncryptedTransport) start() {
    // Wrap each new client in an encrypted wrapper.
    // TODO: have some way of stopping this
    for {
        client := <-t.underlying
        new_client, err := NewEncryptedPacketClient(client, t.secret)
        if err != nil {
            log.Printf("Error starting new encrypted client: %s\n", err)
            continue
        }
        t.accept_ch <- new_client
    }
}

func NewEncryptedTransport(underlying Transport, secret string) (*EncryptedTransport, error) {
    ch := make(chan PacketClient)
    tr := &EncryptedTransport{underlying.AcceptChannel(), ch, secret}

    go tr.start()
    return tr, nil
}
