/*
 * Copyright 2020 Saffat Technologies, Ltd.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package unitdb

import (
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"
)

var (
	dbPath = "test"
)

func cleanup() {
	os.RemoveAll(dbPath)
}

func TestSimple(t *testing.T) {
	cleanup()
	db, err := Open(dbPath, WithBufferSize(1<<4), WithMemdbSize(1<<16), WithFreeBlockSize(1<<16))
	if err != nil {
		t.Fatal(err)
	}

	var i uint16
	var n uint16 = 1000

	contract, err := db.NewContract()
	if err != nil {
		t.Fatal(err)
	}
	topic := []byte("unit1.test")

	if db.internal.dbInfo.count != 0 {
		t.Fatal()
	}

	if data, err := db.Get(NewQuery(topic).WithContract(contract)); data != nil || err != nil {
		t.Fatal()
	}

	if db.internal.dbInfo.count != 0 {
		t.Fatal()
	}

	verifyMsgsAndClose := func() {
		if count := db.Count(); count != uint64(n) {
			if err := db.recoverLog(); err != nil {
				t.Fatal(err)
			}
		}
		var v, vals [][]byte
		v, err = db.Get(NewQuery(append(topic, []byte("?last=1h")...)).WithContract(contract))
		if err != nil {
			t.Fatal(err)
		}
		for i = 0; i < n; i++ {
			val := []byte(fmt.Sprintf("msg.%2d", n-i-1))
			vals = append(vals, val)
		}

		if !reflect.DeepEqual(vals, v) {
			t.Fatalf("expected %v; got %v", vals, v)
		}
		// if size, err := db.FileSize(); err != nil || size == 0 {
		// 	t.Fatal(err)
		// }
		if _, err = db.Varz(); err != nil {
			t.Fatal(err)
		}
		if err := db.Close(); err != nil {
			t.Fatal(err)
		}
	}
	var ids [][]byte

	entry := NewEntry(topic, nil)
	entry.WithContract(contract).WithTTL("1m")
	for i = 0; i < n; i++ {
		messageID := db.NewID()
		entry.WithID(messageID)
		val := []byte(fmt.Sprintf("msg.%2d", i))
		if err := db.PutEntry(entry.WithPayload(val)); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, messageID)
	}
	verifyMsgsAndClose()

	db, err = Open(dbPath, WithMutable())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	_, err = db.Get(NewQuery(topic).WithContract(contract).WithLimit(int(n)))
	if err != nil {
		t.Fatal(err)
	}

	for i = 0; i < n; i++ {
		messageID := db.NewID()
		val := []byte(fmt.Sprintf("msg.%2d", i))
		if err := db.Put(topic, val); err != nil {
			t.Fatal(err)
		}
		if err := db.PutEntry(NewEntry(topic, nil).WithID(messageID).WithPayload(val)); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, messageID)
	}
	if err := db.Sync(); err != nil {
		t.Fatal(err)
	}
	for _, id := range ids {
		db.Delete(id, topic)
	}
}

func TestBatch(t *testing.T) {
	cleanup()
	db, err := Open(dbPath, WithBufferSize(1<<16), WithMemdbSize(1<<16), WithFreeBlockSize(1<<16), WithMutable(), WithBackgroundKeyExpiry())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	contract, err := db.NewContract()
	if err != nil {
		t.Fatal(err)
	}
	topic := []byte("unit2.test")

	var i uint16
	var n uint16 = 100

	verifyMsgsAndClose := func() {
		if count := db.Count(); count != uint64(n) {
			if err := db.recoverLog(); err != nil {
				t.Fatal(err)
			}
		}
		var v, vals [][]byte
		v, err = db.Get(NewQuery(append(topic, []byte("?last=1h")...)).WithContract(contract).WithLimit(int(n)))
		if err != nil {
			t.Fatal(err)
		}
		for i = 0; i < n; i++ {
			val := []byte(fmt.Sprintf("msg.%2d", n-i-1))
			vals = append(vals, val)
		}
		if !reflect.DeepEqual(vals, v) {
			t.Fatalf("expected %v; got %v", vals, v)
		}
		if err := db.Close(); err != nil {
			t.Fatal(err)
		}
	}

	err = db.Batch(func(b *Batch, completed <-chan struct{}) error {
		var ids [][]byte
		for i = 0; i < n; i++ {
			messageID := db.NewID()
			topic := append(topic, []byte("?ttl=1h")...)
			val := []byte(fmt.Sprintf("msg.%2d", i))
			if err := b.PutEntry(NewEntry(topic, val).WithID(messageID).WithContract(contract)); err != nil {
				t.Fatal(err)
			}
			ids = append(ids, messageID)
		}
		return err
	})

	if err != nil {
		t.Fatal(err)
	}
	if err := db.Sync(); err != nil {
		t.Fatal(err)
	}
	verifyMsgsAndClose()
}

func TestExpiry(t *testing.T) {
	cleanup()
	db, err := Open(dbPath, WithMutable(), WithBackgroundKeyExpiry())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	contract, err := db.NewContract()
	if err != nil {
		t.Fatal(err)
	}
	topic := []byte("unit4.test")

	var i uint16
	var n uint16 = 100

	err = db.Batch(func(b *Batch, completed <-chan struct{}) error {
		expiresAt := uint32(time.Now().Add(-1 * time.Hour).Unix())
		entry := &Entry{Topic: topic, ExpiresAt: expiresAt}
		entry.WithContract(contract)
		for i = 0; i < n; i++ {
			val := []byte(fmt.Sprintf("msg.%2d", i))
			if err := db.PutEntry(entry.WithPayload(val)); err != nil {
				t.Fatal(err)
			}
		}
		return err
	})

	if err != nil {
		t.Fatal(err)
	}

	query := NewQuery(topic)
	query.WithContract(contract)
	if data, err := db.Get(query.WithLimit(int(n))); len(data) != 0 || err != nil {
		t.Fatal()
	}
	db.expireEntries()
}

func TestLeasing(t *testing.T) {
	cleanup()
	db, err := Open(dbPath, WithBufferSize(1<<16), WithMemdbSize(1<<16), WithFreeBlockSize(1<<4), WithMutable(), WithBackgroundKeyExpiry())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var i uint16
	var n uint16 = 100

	topic := []byte("unit1.test")
	var ids [][]byte
	for i = 0; i < n; i++ {
		messageID := db.NewID()
		val := []byte(fmt.Sprintf("msg.%2d", i))
		if err := db.PutEntry(NewEntry(topic, val).WithID(messageID)); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, messageID)
	}
	if err := db.Sync(); err != nil {
		t.Fatal(err)
	}
	for _, id := range ids {
		db.Delete(id, topic)
	}
	for i = 0; i < n; i++ {
		messageID := db.NewID()
		val := []byte(fmt.Sprintf("msg.%2d", i))
		if err := db.Put(topic, val); err != nil {
			t.Fatal(err)
		}
		if err := db.PutEntry(NewEntry(topic, val).WithID(messageID)); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, messageID)
	}
	if err := db.Sync(); err != nil {
		t.Fatal(err)
	}
	for _, id := range ids {
		db.Delete(id, topic)
	}
}

func TestWildcardTopics(t *testing.T) {
	cleanup()
	db, err := Open(dbPath, WithBufferSize(1<<16), WithMemdbSize(1<<16), WithFreeBlockSize(1<<16), WithMutable(), WithBackgroundKeyExpiry())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	tests := []struct {
		wtopic []byte
		topic  []byte
		msg    []byte
	}{
		{[]byte("..."), []byte("unit.b.b1"), []byte("...1")},
		{[]byte("unit.b..."), []byte("unit.b.b1.b11.b111.b1111.b11111.b111111"), []byte("unit.b...1")},
		{[]byte("unit.*.b1.b11.*.*.b11111.*"), []byte("unit.b.b1.b11.b111.b1111.b11111.b111111"), []byte("unit.*.b1.b11.*.*.b11111.*.1")},
		{[]byte("unit.*.b1.*.*.*.b11111.*"), []byte("unit.b.b1.b11.b111.b1111.b11111.b111111"), []byte("unit.*.b1.*.*.*.b11111.*.1")},
		{[]byte("unit.b.b1"), []byte("unit.b.b1"), []byte("unit.b.b1.1")},
		{[]byte("unit.b.b1.b11"), []byte("unit.b.b1.b11"), []byte("unit.b.b1.b11.1")},
		{[]byte("unit.b"), []byte("unit.b"), []byte("unit.b.1")},
	}
	for _, tt := range tests {
		db.Put(tt.wtopic, tt.msg)
		if msg, err := db.Get(NewQuery(tt.wtopic).WithLimit(10)); len(msg) == 0 || err != nil {
			t.Fatal(err)
		}
		if msg, err := db.Get(NewQuery(tt.topic).WithLimit(10)); len(msg) == 0 || err != nil {
			t.Fatal(err)
		}
	}
}
