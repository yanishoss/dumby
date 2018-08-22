package protocol

import (
	"encoding/binary"
	"math/rand"
	"testing"
)

func generateRandomBytes(size int64) ([]byte, error) {
	bytes := make([]byte, size)
	_, err := rand.Read(bytes)
	return bytes, err
}

func compareBytes(a *[]byte, b *[]byte) bool {
	if a == b {
		return true
	}

	if len(*a) != len(*b) {
		return false
	}

	stillEqual := true

	for i, el := range *a {
		if el != (*b)[i] {
			stillEqual = false
			break
		}
	}

	return stillEqual
}

func TestSerialize(t *testing.T) {
	sessionSlice, err := generateRandomBytes(32)

	if err != nil {
		t.Error("Cannot generate a random session ID")
	}

	session := Session{}
	copy(session[:], sessionSlice)

	var action Action = 101

	payload := Payload("Hello World!")
	payloadSize := uint64(len(payload))

	trame := &Trame{
		session,
		action,
		payloadSize,
		payload,
	}

	serialBuffer := make([]byte, 48+len(payload))

	trame.Read(serialBuffer)

	dataLen := len(serialBuffer)
	expectedLen := 48 + len(payload)

	if dataLen != expectedLen {
		t.Log("Some data are gone because the size of the serialized data is not the same as the trame's")
		t.Fail()
	}

	serialSession := serialBuffer[:32]
	if !compareBytes(&sessionSlice, &serialSession) {
		t.Log("The original session ID and the serialized one are different")
		t.Fail()
	}

	if binary.LittleEndian.Uint64(serialBuffer[32:40]) != trame.Action {
		t.Log("The original action and the serialized one are different")
		t.Fail()
	}

	serialPayload := serialBuffer[48:]
	tramePayload := []byte(trame.Payload)
	if !compareBytes(&tramePayload, &serialPayload) {
		t.Log("The original payload and the serialized one are different")
		t.Fail()
	}
}

func TestParse(t *testing.T) {
	sessionSlice, err := generateRandomBytes(32)

	if err != nil {
		t.Error("Cannot generate a random session ID")
	}

	session := Session{}
	copy(session[:], sessionSlice)

	var action Action = 101

	payload := Payload("Hello World!")
	payloadSize := uint64(len(payload))

	trame := &Trame{
		Session:     session,
		Action:      action,
		PayloadSize: payloadSize,
		Payload:     payload,
	}

	serialBuffer := make([]byte, 48+len(payload))

	trame.Read(serialBuffer)

	parsedTrame := new(Trame)

	err = Parse(serialBuffer, parsedTrame)

	if err != nil {
		t.Error(err)
	}

	trameSession := trame.Session[:]
	parsedSession := parsedTrame.Session[:]
	if !compareBytes(&trameSession, &parsedSession) {
		t.Log("The original session id and the parsed one are different")
		t.Fail()
	}

	if trame.Action != parsedTrame.Action {
		t.Log("The original action and the parsed one are different")
		t.Fail()
	}

	tramePayload := []byte(trame.Payload)
	parsedPayload := []byte(parsedTrame.Payload)

	if !compareBytes(&tramePayload, &parsedPayload) {
		t.Log("The original payload and the parsed one are different")
		t.Fail()
	}
}
