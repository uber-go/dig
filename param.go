package dig

import (
	"errors"
	"reflect"
)

// Param represents an argument to invoked function.
type Param struct {
	Type     reflect.Type
	Value    interface{}
	Optional bool
}

// Creates a function of one argument that accepts actual val type and sets it to val when executed.
func makeInvoke(tp reflect.Type, val *reflect.Value) interface{} {
	ft := reflect.FuncOf([]reflect.Type{tp}, nil, false)
	return reflect.MakeFunc(ft, func(args []reflect.Value) []reflect.Value {
		val.Set(args[0])
		return nil
	}).Interface()
}

// InvokeWithParams invokes ctor based on container and params.
func InvokeWithParams(container *Container, ctor interface{}, params ...Param) error {
	tCtor := reflect.TypeOf(ctor)
	if tCtor.Kind() != reflect.Func {
		return errors.New("expected a function to invoke")
	}

	// Store all the params values.
	memoize := map[reflect.Type]interface{}{}

	// Check optional parameters first.
	for _, p := range params {
		if p.Value == nil {
			p.Value = reflect.New(p.Type).Elem()
		}

		val := reflect.ValueOf(p.Value)
		invoke := makeInvoke(p.Type, &val)

		if err := container.Invoke(invoke); err != nil && !p.Optional {
			return err
		}

		memoize[p.Type] = p.Value
	}

	// Iterate over inputs and use values saved in optionals, otherwise try to resolve values from the container.
	var ins []reflect.Value
	vCtor := reflect.ValueOf(ctor)
	for i := 0; i < tCtor.NumIn(); i++ {
		tIn := tCtor.In(i)
		if v, ok := memoize[tIn]; ok {
			ins = append(ins, reflect.ValueOf(v))
			continue
		}

		val := reflect.New(tIn).Elem()
		invoke := makeInvoke(tIn, &val)
		if err := container.Invoke(invoke); err != nil {
			return err
		}

		ins = append(ins, val)
	}

	// Evaluate user's ctor.
	vCtor.Call(ins)
	return nil
}
