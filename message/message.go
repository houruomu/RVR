package message

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/binary"
	"fmt"
	"golang.org/x/crypto/sha3"
	"reflect"
	"strconv"
	"unsafe"
)

type Identity struct {
	Address    string
	Public_key []byte
}

func (id *Identity) Size() uintptr{
	var size uintptr
	size += unsafe.Sizeof(id)
	size += uintptr(len(id.Public_key)) * reflect.TypeOf(id.Public_key).Elem().Size()
	return size
}

func (id *Identity) GetUUID() uint64 {
	digest := sha3.Sum224(id.Public_key)
	return binary.LittleEndian.Uint64(digest[0:8])
}

type Message struct {
	Round     int      // Round count
	Sender    Identity // the Identity of the Sender
	Signature []byte   // Signature for the entire Message, all fields should be included
	View      []uint64 // in Sample, this is the View of the Sender, and in Gossip, this is the View of the leader
	Nonce     []byte   // in Elect, challenge and solution header; in Sample, Nonce,
	Proof     [][]byte // the off-path hashes to Elect's puzzle, the first field should be the challenge of the rcver, and
						// the hash of all bytes one be one should be below the intended difficulty
	Order []bool // the order of merging the off-path hashes
	Type  string // indicate the purpose of the message (for easier message handling)
}

func (m *Message) getDigest() []byte {
	hash_tool := sha3.NewShake256()
	hash_tool.Write([]byte(strconv.Itoa(m.Round)))
	hash_tool.Write([]byte(m.Sender.Address))
	hash_tool.Write(m.Sender.Public_key)

	temp := make([]byte, 8)
	for _, id := range m.View {
		binary.LittleEndian.PutUint64(temp, id)
		hash_tool.Write(temp)
	}
	for _, header := range m.Proof {
		hash_tool.Write(header)
	}
	hash_tool.Write(m.Nonce)
	for _, d := range m.Order {
		if d {
			hash_tool.Write([]byte{1})
		} else {
			hash_tool.Write([]byte{0})
		}
	}
	hash_tool.Write([]byte(m.Type))
	digest := make([]byte, 32)
	hash_tool.Read(digest)
	return digest
}

func (m *Message) Verify() error {
	digest := m.getDigest()
	cert, err := x509.ParsePKCS1PublicKey(m.Sender.Public_key)
	if err != nil {
		return err
	}
	err = rsa.VerifyPKCS1v15(cert, crypto.SHA256, digest, m.Signature)
	return err;
}

func (m *Message) Sign(key *rsa.PrivateKey) error {
	m.Sender.Public_key = x509.MarshalPKCS1PublicKey(&(key.PublicKey))
	digest := m.getDigest()
	sigValue, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	m.Signature = sigValue
	return err
}

func (m *Message) String() string {
	return fmt.Sprintf(`
		Round: %d
		Sender: %x
		View Count: %d
		Verified: %t
	`, m.Round, m.Sender.GetUUID(), len(m.View), m.Verify() == nil)
}

func (m *Message) Size() uintptr{
	// compute the size of a message sent over
	// this is necessary for protocol measurement
	var size uintptr
	size += unsafe.Sizeof(m)
	size += m.Sender.Size()
	size += uintptr(len(m.Signature)) * reflect.TypeOf(m.Signature).Elem().Size()
	size += uintptr(len(m.View)) * reflect.TypeOf(m.View).Elem().Size()

	return size
}
