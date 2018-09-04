package message

import (
	"testing"
	"crypto/rsa"
	"crypto/rand"
)

func TestMessage_Sign_Verify(t *testing.T) {
	msg := new(Message)
	msg.Round = 12
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil{
		t.Error(err.Error())
	}
	err = msg.Sign(privateKey)
	if err != nil{
		t.Error(err.Error())
	}
	err = msg.Verify()
	if err != nil{
		t.Error(err.Error())
	}
	msg.Round = 24
	err = msg.Verify()
	if err == nil{
		t.Error("Wrong msg verified!")
	}
}