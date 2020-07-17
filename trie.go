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
	"sync"

	"github.com/unit-io/unitdb/message"
)

const (
	nul = 0x0
)

type topics []topic

// addUnique adds topic to the set.
func (top *topics) addUnique(value topic) (added bool) {
	for i, v := range *top {
		if v.hash == value.hash {
			(*top)[i].offset = value.offset
			return false
		}
	}
	*top = append(*top, value)
	added = true
	return
}

type key struct {
	query     uint32
	wildchars uint8
}

type part struct {
	k        key
	depth    uint8
	parent   *part
	children map[key]*part
	topics   topics
}

func (p *part) orphan() {
	if p.parent == nil {
		return
	}

	delete(p.parent.children, p.k)
	if len(p.parent.children) == 0 {
		p.parent.orphan()
	}
}

// partTrie represents an efficient collection of Trie with lookup capability.
type partTrie struct {
	summary map[uint64]*part // summary is map of topichash to node of tree.
	root    *part            // The root node of the tree.
}

// newPartTrie creates a new part Trie.
func newPartTrie() *partTrie {
	return &partTrie{
		summary: make(map[uint64]*part),
		root: &part{
			children: make(map[key]*part),
		},
	}
}

// trie trie data structure to store topic parts
type trie struct {
	sync.RWMutex
	mutex
	partTrie *partTrie
}

// NewTrie new trie creates a Trie with an initialized Trie.
// Mutex is used to lock concurent read/write on a contract, and it does not lock entire trie.
func newTrie() *trie {
	return &trie{
		mutex:    newMutex(),
		partTrie: newPartTrie(),
	}
}

// Count returns the number of topics in the Trie.
func (t *trie) Count() int {
	t.RLock()
	defer t.RUnlock()
	return len(t.partTrie.summary)
}

// add adds a topic to trie.
func (t *trie) add(topic topic, parts []message.Part, depth uint8) (added bool) {
	// Get mutex
	mu := t.getMutex(topic.hash)
	mu.Lock()
	defer mu.Unlock()
	if _, ok := t.partTrie.summary[topic.hash]; ok {
		return true
	}
	curr := t.partTrie.root
	for _, p := range parts {
		k := key{
			query:     p.Query,
			wildchars: p.Wildchars,
		}
		t.RLock()
		child, ok := curr.children[k]
		t.RUnlock()
		if !ok {
			child = &part{
				k:        k,
				parent:   curr,
				children: make(map[key]*part),
			}
			t.Lock()
			curr.children[k] = child
			t.Unlock()
		}
		curr = child
	}
	t.Lock()
	curr.topics.addUnique(topic)
	t.partTrie.summary[topic.hash] = curr
	t.Unlock()
	added = true
	curr.depth = depth
	return
}

// lookup returns window entry set for given topic.
func (t *trie) lookup(query []message.Part, depth, topicType uint8) (tops topics) {
	t.RLock()
	defer t.RUnlock()
	// fmt.Println("trie.lookup: depth, parts ", depth, query)
	t.ilookup(query, depth, topicType, &tops, t.partTrie.root)
	return
}

func (t *trie) ilookup(query []message.Part, depth, topicType uint8, tops *topics, currpart *part) {
	// Add topics from the current branch
	if currpart.depth == depth || (topicType == message.TopicStatic && currpart.k.query == message.Wildcard) {
		for _, topic := range currpart.topics {
			tops.addUnique(topic)
		}
	}

	// if done then stop
	if len(query) == 0 {
		return
	}

	q := query[0]
	// Go through the wildcard match branch
	for k, p := range currpart.children {
		switch {
		case k.query == q.Query && q.Wildchars == k.wildchars:
			// fmt.Println("trie.lookup: wildchars, part ", k.wildchars, k.query)
			t.ilookup(query[1:], depth, topicType, tops, p)
		case k.query == q.Query && uint8(len(query)) >= k.wildchars+1:
			// fmt.Println("trie.lookup: wildchar, part ", k.wildchars, k.query)
			t.ilookup(query[k.wildchars+1:], depth, topicType, tops, p)
		case k.query == message.Wildcard:
			// fmt.Println("trie.lookup: wildcard, part ", k.query)
			t.ilookup(query[:], depth, topicType, tops, p)
		}
	}
}

func (t *trie) getOffset(topicHash uint64) (off int64, ok bool) {
	t.RLock()
	defer t.RUnlock()
	if curr, ok := t.partTrie.summary[topicHash]; ok {
		for _, topic := range curr.topics {
			if topic.hash == topicHash {
				return topic.offset, ok
			}
		}
	}
	return off, ok
}

func (t *trie) setOffset(top topic) (ok bool) {
	t.Lock()
	defer t.Unlock()
	if curr, ok := t.partTrie.summary[top.hash]; ok {
		curr.topics.addUnique(top)
		return ok
	}
	return false
}
