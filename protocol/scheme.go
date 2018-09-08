/*

This package provides tools for:
	- serializing trames
	- parsing trames

Nothing is really difficult so go ahead and read the code!

*/

package protocol

import (
	"encoding/binary"
	"errors"
)

const (
	// HeaderSize is the size of a Trame's header.
	HeaderSize = 48
	// MaxTrameSize is the maximal size of a Trame including the header.
	MaxTrameSize = 4096
)

const (
	// ActionInit is the action sent at the initialization of the session.
	ActionInit = iota + 1
)

// Session is a 256 bits ID.
type Session = [32]byte

// Action is a 64 bits ID describing the action requested.
type Action = uint64

// PayloadSize is the size of the Payload.
type PayloadSize = uint64

// Payload is the data of the Trame.
type Payload = []byte

// Trame is a Dumby trame.
// It's pretty straightforward, no need to describe it more.
type Trame struct {
	Session     Session
	Action      Action
	PayloadSize PayloadSize
	Payload     Payload
}

func bytesToUint64(data *[]byte) (uint64, error) {
	if len(*data) < 8 {
		return 0, errors.New("The data are too short for being an int64")
	}

	return binary.LittleEndian.Uint64(*data), nil
}

// New creates a Trame.
func New(session Session, action Action, payload Payload) *Trame {
	if payload == nil {
		payload = make([]byte, 0)
	}

	return &Trame{
		Session:     session,
		Action:      action,
		PayloadSize: uint64(len(payload)),
		Payload:     payload,
	}
}

// Parse converts bytes to a Trame.
func Parse(data []byte, trame *Trame) error {
	dataLen := len(data)

	if dataLen < 48 {
		return errors.New("The data are too short for being a correct trame")
	}

	sessionBuffer := Session{}
	copy(sessionBuffer[:], (data)[:32])

	actionBuffer := (data)[32:40]
	action, err := bytesToUint64(&actionBuffer)

	if err != nil {
		return errors.New("Cannot parse the trame's action")
	}

	payloadSizeBuffer := (data)[40:48]
	payloadSize, err := bytesToUint64(&payloadSizeBuffer)

	if err != nil {
		return errors.New("Cannot parse the trame's payload size")
	}

	if dataLen < int(48+payloadSize) {
		return errors.New("The payload size specified is incorrect")
	}

	payload := (data)[48 : 48+payloadSize]

	trame.Session = sessionBuffer
	trame.Action = action
	trame.PayloadSize = payloadSize
	trame.Payload = payload

	return nil
}

func (trame *Trame) Read(buffer []byte) int {
	serialBuffer := make([]byte, 48+len(trame.Payload))

	copy(serialBuffer, (trame.Session)[:])

	binary.LittleEndian.PutUint64(serialBuffer[32:40], trame.Action)

	binary.LittleEndian.PutUint64(serialBuffer[40:48], trame.PayloadSize)

	copy(serialBuffer[48:48+trame.PayloadSize], trame.Payload)

	copy(buffer, serialBuffer)

	return len(serialBuffer)
}
