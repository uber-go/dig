package dig

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStringer(t *testing.T) {
	c := New()
	type A struct{}
	type B struct{}
	require.NoError(t, c.Provide(func() (*A, *B) { return &A{}, &B{} }))
	require.NoError(t, c.Invoke(func(a *A) {}))

	b := &bytes.Buffer{}
	fmt.Fprintln(b, c)
	s := b.String()

	assert.Contains(t, s, "*dig.A ->")
	assert.Contains(t, s, "*dig.B ->")
	assert.Contains(t, s, "func() (*dig.A, *dig.B)")

	assert.Contains(t, s, "*dig.A =>")
	assert.Contains(t, s, "*dig.B =>")
}
