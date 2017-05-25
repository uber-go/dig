package vallie

import (
	"reflect"

	"github.com/pkg/errors"
)

var errZeroVal = errors.New("Object is a zero value")

// Validate accepts objects to be validated from constructor and returns an error
// if any of the provided object is a zero value object
func Validate(values ...interface{}) error {
	for _, val := range values {
		v := reflect.ValueOf(val)
		if v == reflect.Zero(v.Type()) {
			return errors.Wrapf(errZeroVal, "%v", v.Type())
		}
	}
	return nil
}
