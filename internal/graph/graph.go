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

package graph

import (
	"bytes"
	"fmt"
	"reflect"
	"sync"

	"github.com/pkg/errors"
)

var (
	errArgKind = errors.New("constructor arguments must be pointers")

	_typeOfError = reflect.TypeOf((*error)(nil)).Elem()
)

// Graph contains all Graph for current graph
type Graph struct {
	sync.RWMutex

	nodes map[interface{}]graphNode
}

// NewGraph creates new data Graph for dig
func NewGraph() Graph {
	return Graph{
		nodes: make(map[interface{}]graphNode),
	}
}

// Reset the graph
func (g *Graph) Reset() {
	g.Lock()
	defer g.Unlock()
	g.nodes = make(map[interface{}]graphNode)
}

// Read reads value from the Graph
func (g *Graph) Read(objType reflect.Type) (reflect.Value, error) {
	// check if the type is a registered objNode
	g.RLock()
	n, ok := g.nodes[objType]
	g.RUnlock()
	if !ok {
		return reflect.Zero(objType), fmt.Errorf("type %v is not registered", objType)
	}
	v, err := n.value(g, objType)
	if err != nil {
		return reflect.Zero(objType), errors.Wrapf(err, "unable to resolve %v", objType)
	}
	return v, nil
}

// InsertObject the Graph with the provided value
func (g *Graph) InsertObject(v reflect.Value) error {
	g.Lock()
	defer g.Unlock()
	onode := objNode{
		node: node{
			objType:     v.Type(),
			cached:      true,
			cachedValue: v,
		},
	}
	g.nodes[v.Type()] = &onode
	return nil
}

// InsertConstructor adds the constructor to the Graph
func (g *Graph) InsertConstructor(ctor interface{}) error {
	ctype := reflect.TypeOf(ctor)
	// count of number of objects to be registered from the list of return parameters
	count := ctype.NumOut()
	objTypes := make([]reflect.Type, count, count)
	for i := 0; i < count; i++ {
		objTypes[i] = ctype.Out(i)
	}

	nodes := make([]node, count, count)
	for i := 0; i < count; i++ {
		nodes[i] = node{
			objType: objTypes[i],
		}
	}
	argc := ctype.NumIn()
	n := funcNode{
		deps:        make([]interface{}, argc),
		constructor: ctor,
		nodes:       nodes,
	}
	for i := 0; i < argc; i++ {
		arg := ctype.In(i)
		if arg.Kind() != reflect.Ptr && arg.Kind() != reflect.Interface {
			return errArgKind
		}
		n.deps[i] = arg
	}

	g.Lock()
	defer g.Unlock()
	for i := 0; i < count; i++ {
		g.nodes[objTypes[i]] = &n
	}

	// object needs to be part of the container to properly detect cycles
	if cycleErr := g.recursiveDetectCycles(&n, nil); cycleErr != nil {
		// if the cycle was detected delete from the container
		for objType := range objTypes {
			delete(g.nodes, objType)
		}
		return errors.Wrapf(cycleErr, "unable to Provide %v", objTypes)
	}

	return nil
}

// DFS and tracking if same node is visited twice
func (g *Graph) recursiveDetectCycles(n graphNode, l []string) error {
	for _, el := range l {
		if n.id() == el {
			b := &bytes.Buffer{}
			for _, curr := range l {
				fmt.Fprint(b, curr, " -> ")
			}
			fmt.Fprint(b, n.id())
			return fmt.Errorf("detected cycle %s", b.String())
		}
	}

	l = append(l, n.id())

	for _, dep := range n.dependencies() {
		if node, ok := g.nodes[dep]; ok {
			if err := g.recursiveDetectCycles(node, l); err != nil {
				return err
			}
		}
	}
	return nil
}

func (g *Graph) validateGraph(ct reflect.Type) (reflect.Value, error) {
	g.Lock()
	defer g.Unlock()

	for _, node := range g.nodes {
		for _, dep := range node.dependencies() {
			// check that the dependency is a registered objNode
			if _, ok := g.nodes[dep]; !ok {
				return reflect.Zero(ct), fmt.Errorf("%v dependency of type %v is not registered", ct, dep)
			}
		}
	}
	return reflect.Zero(ct), nil
}

// ConstructorArguments returns arguments in the provided constructor
func (g *Graph) ConstructorArguments(ctype reflect.Type) ([]reflect.Value, error) {
	// find dependencies from the graph and place them in the args
	args := make([]reflect.Value, ctype.NumIn(), ctype.NumIn())
	for idx := range args {
		arg := ctype.In(idx)
		g.RLock()
		node, ok := g.nodes[arg]
		g.RUnlock()
		if ok {
			v, err := node.value(g, arg)
			if err != nil {
				return nil, errors.Wrapf(err, "unable to resolve %v", arg)
			}
			args[idx] = v
		} else {
			return nil, fmt.Errorf("%v dependency of type %v is not registered", ctype, arg)
		}
	}
	return args, nil
}

// String representation of the entire Container
func (g *Graph) String() string {
	b := &bytes.Buffer{}
	fmt.Fprintln(b, "{nodes:")
	for key, reg := range g.nodes {
		fmt.Fprintln(b, key, "->", reg)
	}
	fmt.Fprintln(b, "}")
	return b.String()
}
