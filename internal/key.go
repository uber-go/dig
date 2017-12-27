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

import (
	"fmt"
	"reflect"
)

// Key identifies an entry in the Container.
type Key interface {
	fmt.Stringer

	keyType()

	GetType() reflect.Type
}

// ValueKey identifies a single type in the Container.
type ValueKey struct {
	// Name of the value, if any.
	Name string

	// Type of the value.
	Type reflect.Type
}

var _ Key = ValueKey{}

func (ValueKey) keyType() {}

func (k ValueKey) String() string {
	if len(k.Name) > 0 {
		return fmt.Sprintf("%v[name=%q]", k.Type, k.Name)
	}
	return fmt.Sprint(k.Type)
}

// GetType returns the type of value.
func (k ValueKey) GetType() reflect.Type { return k.Type }

// GroupKey identifies a value group in the Container.
type GroupKey struct {
	// Name of the value group. Required.
	Name string

	// Type of values in the value group.
	//
	// Note that this is the type of the value, not the slice in which this
	// value be stored.
	Type reflect.Type
}

var _ Key = GroupKey{}

func (GroupKey) keyType() {}

func (k GroupKey) String() string {
	return fmt.Sprintf("%v[group=%q]", k.Type, k.Name)
}

// GetType returns the type of value.
func (k GroupKey) GetType() reflect.Type { return k.Type }
