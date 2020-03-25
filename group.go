// Copyright (c) 2020 Uber Technologies, Inc.
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

package dig

import (
	"reflect"
	"strings"
)

const (
	_groupTag = "group"
)

type group struct {
	Name    string
	Flatten bool
}

func parseGroupTag(f reflect.StructField) (group, error) {
	tag := f.Tag.Get(_groupTag)
	if tag == "" {
		panic("It looks like you have found a bug in dig. " +
			"Please file an issue at https://github.com/uber-go/dig/issues/ " +
			"and provide the following message: " +
			"parseGroupTag() must never be called without group tag")
	}

	components := strings.Split(tag, ",")
	g := group{Name: components[0]}
	for _, c := range components[1:] {
		switch c {
		case "flatten":
			g.Flatten = true
		default:
			return group{}, errf(
				"invalid option %q for %q tag on field %v",
				c, _groupTag, f.Name)
		}
	}
	return g, nil
}
