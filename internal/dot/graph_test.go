package dot

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNodeString(t *testing.T) {
	n1 := &Node{Type: "t1"}
	n2 := &Node{Type: "t2", Name: "bar"}
	n3 := &Node{Type: "t3", Group: "foo"}

	assert.Equal(t, "t1", n1.String())
	assert.Equal(t, "t2[name=bar]", n2.String())
	assert.Equal(t, "t3[group=foo]", n3.String())
}

func TestAttributes(t *testing.T) {
	n1 := &Node{Type: "t1"}
	n2 := &Node{Type: "t2", Name: "bar"}
	n3 := &Node{Type: "t3", Group: "foo"}

	assert.Equal(t, "", n1.Attributes())
	assert.Equal(t, `<BR /><FONT POINT-SIZE="10">Name: bar</FONT>`, n2.Attributes())
	assert.Equal(t, `<BR /><FONT POINT-SIZE="10">Group: foo</FONT>`, n3.Attributes())
}
