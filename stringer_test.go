package dig

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStringer(t *testing.T) {
	t.Parallel()

	c := New()
	type A struct{}
	type B struct{}
	type C struct{}
	require.NoError(t, c.Provide(func() (*A, *B) { return &A{}, &B{} }))
	require.NoError(t, c.Provide(func(*B) *C { return &C{} }))
	require.NoError(t, c.Invoke(func(a *A) {}))

	b := &bytes.Buffer{}
	fmt.Fprintln(b, c)
	s := b.String()

	// all nodes are in the graph
	assert.Contains(t, s, "*dig.A -> deps: []")
	assert.Contains(t, s, "*dig.B -> deps: []")
	assert.Contains(t, s, "*dig.C -> deps: [*dig.B]")

	// constructors
	assert.Contains(t, s, "func() (*dig.A, *dig.B)")
	assert.Contains(t, s, "func(*dig.B) *dig.C")

	// cache
	assert.Contains(t, s, "*dig.A =>")
	assert.Contains(t, s, "*dig.B =>")
}
