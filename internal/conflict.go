// Copyright (c) 2018 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package internal

import "fmt"

// KeySet is an ordered set of keys. KeySet enforces that all keys in it are
// usable together.
//
// Keys are considered to be usable together if for each pair x, y of keys,
// one of the following holds true.
//
//   - x and y have different types
//   - x and y have different names
//   - x or y is a group key
type KeySet struct {
	// ValueKeys that have already been taken. The value for this map is the
	// source from which this key came.
	taken map[ValueKey]string
	keys  []Key
}

// NewKeySet builds a new KeySet.
func NewKeySet() *KeySet {
	return &KeySet{taken: make(map[ValueKey]string)}
}

// Provide attempts to add the given key to the KeySet. An error is returned
// if adding this Key would cause a conflict. source specifies a user-friendly
// name of the source from where this Key was provided. This will be used in
// error messages if another source attempts to add the same key.
func (ks *KeySet) Provide(source string, k Key) error {
	// We need to worry about conflicts only if we're dealing with a ValueKey.
	vk, ok := k.(ValueKey)
	if !ok {
		ks.keys = append(ks.keys, k)
		return nil
	}

	if conflict, ok := ks.taken[vk]; ok {
		return fmt.Errorf("already provided by %v", conflict)
	}

	ks.taken[vk] = source
	ks.keys = append(ks.keys, k)
	return nil
}

// Items returns a list of items in this KeySet.
func (ks *KeySet) Items() []Key { return ks.keys }
