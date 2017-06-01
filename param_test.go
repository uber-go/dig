package dig

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInvokeWithParams(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	c := New()
	type hero struct {
		name string
	}
	type cartoon struct {
		title string
	}

	assert.NoError(c.Provide(func() *hero { return &hero{name: "superman"} }))
	err := InvokeWithParams(
		c,
		func(h *hero, ct *cartoon) {
			assert.Equal("superman", h.name)
			assert.Equal("Snafuperman", ct.title)
		},
		Param{
			Type:     reflect.TypeOf(&cartoon{}),
			Optional: true,
			Value:    &cartoon{title: "Snafuperman"},
		})

	assert.NoError(err)
}
