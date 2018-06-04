package dig

import (
	"bytes"
	"flag"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var generate = flag.Bool("generate", false, "generates output to testdata/ if set")

func VerifyVisualization(t *testing.T, testname string, c *Container) {
	flag.Parse()

	var b bytes.Buffer
	require.NoError(t, Visualize(c, &b))

	if *generate {
		err := ioutil.WriteFile("testdata/"+testname+".dot", b.Bytes(), 0644)
		require.NoError(t, err)
		return
	}

	wantBytes, err := ioutil.ReadFile("testdata/" + testname + ".dot")
	require.NoError(t, err)

	got := b.String()
	want := string(wantBytes)
	assert.Equal(t, want, got,
		"Output did not match. Make sure you updated the testdata by running 'go test -generate'")
}
