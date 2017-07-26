// Copyright (c) 2017 Uber Technologies, Inc.
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
	"fmt"
	"runtime"
	"strings"
)

// FrameSkipper allows for more fine-grained configuration on which stack frames
// should be ignored to expose the true caller.
type FrameSkipper func(f runtime.Frame) bool

// DefaultFrameSkipper provides a default implementation of FrameSkipper
// that is sufficient when dig is used directly without Provide and Invoke
// methods being wrapped.
func defaultFrameSkipper(f runtime.Frame) bool {
	// When running tests, it's helpful not to skip frames to expose
	// the top of the stack caller.
	if strings.Contains(f.File, "_test.go") {
		return false
	}

	// Remove all traces of internal dig methods. This makes sure that this
	// function does not need to be modified even during dig refactors.
	if strings.Contains(f.File, "go.uber.org/dig") {
		return true
	}

	return false
}

// Returns a formatted calling func name and line number.
//
// It uses the passed in FrameSkipper to get to what's considered to be
// true source of the call.
func getCaller(skipper FrameSkipper) string {
	// Ascend at most 8 frames looking for a caller outside dig.
	pcs := make([]uintptr, 8)

	// Skip some frames from the top:
	// 1 to skip the runtime.Callers itself, as per docs
	// 2 to skip the current function
	n := runtime.Callers(2, pcs)

	if n > 0 {
		frames := runtime.CallersFrames(pcs)
		for f, more := frames.Next(); more; f, more = frames.Next() {
			if skipper(f) {
				continue
			}
			return fmt.Sprintf("%s:%d", f.Function, f.Line)
		}
	}

	return "n/a"
}
