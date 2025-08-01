package wam

import (
	"encoding/binary"

	"go.mau.fi/whatsmeow/binary/wam/wamschema"
)

type EventMeta struct {
	ID      int
	Channel string
	Weight  int
	StatsID int
}

type Event struct {
	Event   any
	Globals wamschema.WAMGlobals
}

type WAMEncoder struct {
	version  int
	sequence int
	events   []Event
	data     []byte
}

func NewDefaultWAMEncoder() *WAMEncoder { return &WAMEncoder{data: []byte{}, sequence: 1, version: 5} }

func (wam *WAMEncoder) writeHeader() {
	headerBytes := make([]byte, 8)

	copy(headerBytes, "WAM") // header

	headerBytes[3] = byte(wam.version) // version of the protocol

	headerBytes[4] = 1 // random ?

	binary.BigEndian.PutUint16(headerBytes[5:], uint16(wam.sequence)) // wam buffer sequence

	headerBytes[7] = 0 // channel

	wam.data = append(wam.data, headerBytes...)
}

func (wam *WAMEncoder) PutEvent(event Event) {
	wam.events = append(wam.events, event)
}

func (wam *WAMEncoder) writeEvent(event Event) {
	eventBytes := make([]byte, 0)
	meta := extractEventMeta(event.Event)
	extended := isExtended(event.Event)

	// write globals
	eventBytes = append(eventBytes, encodeGlobalAttributes(event.Globals)...)

	// write event header
	eventBytes = append(eventBytes, encodeEventHeader(meta, extended)...)

	// write props
	eventBytes = append(eventBytes, encodeEventProps(event.Event)...)

	wam.data = append(wam.data, eventBytes...)
}

func (wam *WAMEncoder) GetData() []byte {
	wam.writeHeader()

	for _, event := range wam.events {
		wam.writeEvent(event)
	}

	return wam.data
}
