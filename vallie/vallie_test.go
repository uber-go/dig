package vallie

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidation(t *testing.T) {
	type nothing struct{}

	tests := []struct {
		desc  string
		input []interface{}
		valid bool
	}{
		{"empty input", []interface{}{}, true},
		{"nil pointer", []interface{}{(*nothing)(nil)}, false},
		{"zero value", []interface{}{nothing{}}, false},
		{"non-zero value", []interface{}{5}, true},
		{"mixed input", []interface{}{5, http.DefaultServeMux}, true},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			assert.Equal(
				t,
				tt.valid,
				Validate(tt.input...) == nil,
				"unexpected validation result for %v", tt.input,
			)
		})
	}
}
