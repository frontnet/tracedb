package tracedb

import (
	"strconv"
	"time"
	"unsafe"
)

// Entry represents an entry which is stored into DB.
type Entry struct {
	contract   uint64
	seq        uint64
	id         []byte
	topicHash  uint64
	topic      []byte
	val        []byte
	encryption bool
	parsed     bool
	ID         []byte `json:"id,omitempty"`   // The ID of the message
	Topic      []byte `json:"chan,omitempty"` // The topic of the message
	Payload    []byte `json:"data,omitempty"` // The payload of the message
	ExpiresAt  uint32 // The time expiry of the message
	Contract   uint32 // The contract is used to as salt to hash topic parts and also used as prefix in the message Id
}

// NewEntry creates a new entry structure from the topic and payload.
func NewEntry(topic, payload []byte) *Entry {
	return &Entry{
		Topic:   topic,
		Payload: payload,
	}
}

func (e *Entry) SetID(id []byte) *Entry {
	e.ID = id
	return e
}

func (e *Entry) SetPayload(payload []byte) *Entry {
	e.Payload = payload
	return e
}

func (e *Entry) SetContract(contract uint32) *Entry {
	e.Contract = contract
	return e
}

func (e *Entry) SetTTL(ttl []byte) *Entry {
	val, err := strconv.ParseInt(unsafeToString(ttl), 10, 64)
	if err == nil {
		e.ExpiresAt = uint32(time.Now().Add(time.Duration(int(val)) * time.Second).Unix())
		return e
	}
	var duration time.Duration
	duration, _ = time.ParseDuration(unsafeToString(ttl))
	e.ExpiresAt = uint32(time.Now().Add(duration).Unix())
	return e
}

// unsafeToString is used to convert a slice
// of bytes to a string without incurring overhead.
func unsafeToString(bs []byte) string {
	return *(*string)(unsafe.Pointer(&bs))
}
