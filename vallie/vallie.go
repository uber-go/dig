// Package vallie stands in for a stand-alone validation library
// purely for discussion purposes.
//
// Assuming this is a desired approach, more fidelity will be added
// and vallie would be extracted into it's own repo
package vallie

import (
	"reflect"

	"github.com/pkg/errors"
)

var errZeroVal = errors.New("Object is a zero value")

// Validate accepts objects to be validated from constructor and returns an error
// if any of the provided object is a zero value object
//
// TODO: if Validate returns an error, how can we hook into the dig resolution
// cycle to produce a meaningful error message, i.e.
// "failed to resolve Type3, because non-optional Type2 is not registered"
func Validate(values ...interface{}) error {
	for _, val := range values {
		v := reflect.ValueOf(val)
		if v == reflect.Zero(v.Type()) {
			return errors.Wrapf(errZeroVal, "%v", v.Type())
		}
	}
	return nil
}
