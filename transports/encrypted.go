package transports

import (
    "crypto/aes"
    "crypto/cipher"
    "crypto/hmac"
    "crypto/rand"
    "crypto/sha256"
    "crypto/subtle"
    "fmt"
    "hash"
    "io"
    "log"
    "time"

    "code.google.com/p/go.crypto/nacl/secretbox"
    "code.google.com/p/go.crypto/pbkdf2"
)

// This package implements a simple encrypted transport on top of an existing
// transport.  In general, there's two modes of operation:
//      - For reliable transports (e.g. TCP), it runs AES in CTR mode over all
//        the data that's being sent and received, maintaining one cipher
//        context for each direction of transport.  The message is
//        authenticated using HMAC.
//
//      - For unreliable transports, we can't assume anything about the context
//        of an individual packet, so we use go.crypto's secretbox package and
//        random nonces (that are included with the packet itself).  Note that
//        this package both encrypts and authenticates, we do not HMAC.
//
// Encryption keys are generated from a shared secret that must be provided.
// Note that this doesn't (necessarily) need to be the same as the
// authentication secret, just that it must be equal on both side of the
// connection.
//
// Since the underlying machinery is mostly the same, we implement the basic
// functionality as a structure, and provide the modes of operation as functions
// that are somewhat black boxes.

type encryptionMode interface {
    Encrypt(data []byte) []byte
    Decrypt(data []byte) ([]byte, bool)
}

type aesMode struct {
    stream cipher.Stream
    mac    hash.Hash
}

type secretboxMode struct {
    key [32]byte
}

type EncryptedPacketClient struct {
    underlying   PacketClient
    send_mode   encryptionMode
    recv_mode encryptionMode
    send_ch      chan []byte
    recv_ch      chan []byte
    key          []byte
}

const TEST_STRING = "this is a test string"

// --------------------------------------------------------------------------------

func (m *aesMode) Encrypt(input []byte) []byte {
    output := make([]byte, len(input))
    m.stream.XORKeyStream(output, input)

    // Start from reset each time.
    m.mac.Reset()
    m.mac.Write(output)

    output = m.mac.Sum(output)
    return output
}

func (m *aesMode) Decrypt(encrypted []byte) ([]byte, bool) {
    if len(encrypted) < 32 {
        return nil, false
    }
    output := make([]byte, len(encrypted) - 32)

    // Short forms!
    data := encrypted[0:len(encrypted) - 32]
    mac := encrypted[len(encrypted) - 32:]

    // Validate HMAC.
    m.mac.Reset()
    m.mac.Write(data)
    expected := m.mac.Sum(nil)

    if subtle.ConstantTimeCompare(expected, mac) != 1 {
        return nil, false
    }

    m.stream.XORKeyStream(output, data)
    return output, true
}

func NewAesMode(key []byte) (*aesMode, error) {
    // IV is defined as all 0s - it doesn't need to be secret.
    iv := make([]byte, aes.BlockSize)
    block, err := aes.NewCipher(key)
    if err != nil {
        return nil, err
    }
    stream := cipher.NewCTR(block, iv)
    hm := hmac.New(sha256.New, key)

    return &aesMode{stream, hm}, nil
}

// --------------------------------------------------------------------------------

func (m *secretboxMode) Encrypt(input []byte) []byte {
    var out []byte
    var nonce [24]byte

    n, err := io.ReadFull(rand.Reader, nonce[:])
    if n != len(nonce) || err != nil {
        panic("could not read from random number generator")
    }

    out = secretbox.Seal(out[:0], input, &nonce, &m.key)
    out = append(out, nonce[:]...)
    return out
}

func (m *secretboxMode) Decrypt(encrypted []byte) ([]byte, bool) {
    var opened []byte
    var nonce [24]byte

    if len(encrypted) < 24 {
        return nil, false
    }

    for i := 0; i < 24; i++ {
        nonce[i] = encrypted[len(encrypted) - 24 + i]
    }

    opened, ok := secretbox.Open(opened[:0], encrypted, &nonce, &m.key)
    return opened, ok
}

func NewSecretBoxMode(key []byte) (*secretboxMode, error) {
    var key_arr [32]byte
    if len(key) != 32 {
        return nil, fmt.Errorf("invalid key length (%d != 32)", len(key))
    }

    // TODO: this is ugly, there's got to be a better way
    for i := 0; i < 32; i++ {
        key_arr[i] = key[i]
    }
    return &secretboxMode{key_arr}, nil
}

// --------------------------------------------------------------------------------

func NewEncryptedPacketClient(underlying PacketClient, secret string) (*EncryptedPacketClient, error) {
    var err error

    // PBKDF2 the key to the appropriate size of 32 bytes (both for secretbox
    // and for AES).
    key := pbkdf2.Key([]byte(secret), []byte{}, 16384, 32, sha256.New)

    // Depending on whether the underlying transport is reliable or not, we
    // create a different mode.  We also do this twice, since we need one for
    // each direction.
    var send_mode encryptionMode
    var recv_mode encryptionMode

    if underlying.IsReliable() {
        send_mode, err = NewAesMode(key)
        if err != nil {
            return nil, err
        }

        recv_mode, err = NewAesMode(key)
        if err != nil {
            return nil, err
        }
    } else {
        send_mode, err = NewSecretBoxMode(key)
        if err != nil {
            return nil, err
        }

        recv_mode, err = NewSecretBoxMode(key)
        if err != nil {
            return nil, err
        }
    }

    // Good, have our modes.  We set up our client now...
    ret := &EncryptedPacketClient{
        underlying, send_mode, recv_mode,
        make(chan []byte), make(chan []byte),
        key,
    }

    go ret.doSend()
    go ret.doRecv()

    // Now, we need to authenticate it.  We do this simply by sending an
    // encrypted constant (above), and waiting for a message from the server.
    // If we get a message that decrypts to the same constant, then we assume
    // that everything is legit.
    timeout := time.After(10 * time.Second)

    var test_bytes = []byte(TEST_STRING)
    ret.SendChannel() <- test_bytes

    select {
    case msg := <- ret.RecvChannel():
        // Verify it matches.
        if subtle.ConstantTimeCompare(msg, test_bytes) != 1 {
            log.Printf("Received invalid message\n")
            return nil, fmt.Errorf("invalid message from remote end")
        }

        // Fall through to success
    case <-timeout:
        // Nope, errored!
        log.Printf("Authentication timed out\n")
        ret.Close()
        return nil, fmt.Errorf("authentication timed out")
    }

    return ret, nil
}

func (c *EncryptedPacketClient) doSend() {
    mode := c.send_mode
    ch := c.send_ch
    underlying := c.underlying.SendChannel()

    // TODO: have some way of stopping this
    for {
        unenc := <-ch
        enc := mode.Encrypt(unenc)
        underlying <- enc
    }
}

func (c *EncryptedPacketClient) doRecv() {
    stream := c.recv_mode
    ch := c.recv_ch
    underlying := c.underlying.RecvChannel()

    // TODO: have some way of stopping this
    for {
        enc := <-underlying
        unenc, good := stream.Decrypt(enc)
        if !good {
            log.Printf("Error decrypting packet, skipping...\n")
            continue
        }
        ch <- unenc
    }
}

func (c *EncryptedPacketClient) SendChannel() chan []byte {
    return c.send_ch
}

func (c *EncryptedPacketClient) RecvChannel() chan []byte {
    return c.recv_ch
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
