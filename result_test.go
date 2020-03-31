// Copyright (c) 2019 Uber Technologies, Inc.
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

package dig

import (
	"fmt"
	"io"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewResultListErrors(t *testing.T) {
	tests := []struct {
		desc string
		give interface{}
	}{
		{
			desc: "returns dig.In",
			give: func() struct{ In } { panic("invalid") },
		},
		{
			desc: "returns dig.Out+dig.In",
			give: func() struct {
				Out
				In
			} {
				panic("invalid")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			_, err := newResultList(reflect.TypeOf(tt.give), resultOptions{})
			require.Error(t, err)
			assertErrorMatches(t, err,
				"bad result 1:",
				"cannot provide parameter objects:",
				"embeds a dig.In")
		})
	}
}

func TestResultListExtractFails(t *testing.T) {
	rl, err := newResultList(reflect.TypeOf(func() (io.Writer, error) {
		panic("function should not be called")
	}), resultOptions{})
	require.NoError(t, err)
	assert.Panics(t, func() {
		rl.Extract(newStagingContainerWriter(), reflect.ValueOf("irrelevant"))
	})
}

func TestNewResultErrors(t *testing.T) {
	type outPtr struct{ *Out }
	type out struct{ Out }
	type in struct{ In }
	type inOut struct {
		In
		Out
	}

	tests := []struct {
		give interface{}
		err  string
	}{
		{
			give: outPtr{},
			err:  "cannot build a result object by embedding *dig.Out, embed dig.Out instead: dig.outPtr embeds *dig.Out",
		},
		{
			give: (*out)(nil),
			err:  "cannot return a pointer to a result object, use a value instead: *dig.out is a pointer to a struct that embeds dig.Out",
		},
		{
			give: in{},
			err:  "cannot provide parameter objects: dig.in embeds a dig.In",
		},
		{
			give: inOut{},
			err:  "cannot provide parameter objects: dig.inOut embeds a dig.In",
		},
	}

	for _, tt := range tests {
		give := reflect.TypeOf(tt.give)
		t.Run(fmt.Sprint(give), func(t *testing.T) {
			_, err := newResult(give, resultOptions{})
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.err)
		})
	}
}

func TestNewResultObject(t *testing.T) {
	typeOfReader := reflect.TypeOf((*io.Reader)(nil)).Elem()
	typeOfWriter := reflect.TypeOf((*io.Writer)(nil)).Elem()

	tests := []struct {
		desc string
		give interface{}
		opts resultOptions

		wantFields []resultObjectField
	}{
		{desc: "empty", give: struct{ Out }{}},
		{
			desc: "multiple values",
			give: struct {
				Out

				Reader io.Reader
				Writer io.Writer
			}{},
			wantFields: []resultObjectField{
				{
					FieldName:  "Reader",
					FieldIndex: 1,
					Result:     resultSingle{Type: typeOfReader},
				},
				{
					FieldName:  "Writer",
					FieldIndex: 2,
					Result:     resultSingle{Type: typeOfWriter},
				},
			},
		},
		{
			desc: "name tag",
			give: struct {
				Out

				A io.Writer `name:"stream-a"`
				B io.Writer `name:"stream-b" `
			}{},
			wantFields: []resultObjectField{
				{
					FieldName:  "A",
					FieldIndex: 1,
					Result:     resultSingle{Name: "stream-a", Type: typeOfWriter},
				},
				{
					FieldName:  "B",
					FieldIndex: 2,
					Result:     resultSingle{Name: "stream-b", Type: typeOfWriter},
				},
			},
		},
		{
			desc: "group tag",
			give: struct {
				Out

				Writer io.Writer `group:"writers"`
			}{},
			wantFields: []resultObjectField{
				{
					FieldName:  "Writer",
					FieldIndex: 1,
					Result:     resultGrouped{Group: "writers", Type: typeOfWriter},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			got, err := newResultObject(reflect.TypeOf(tt.give), tt.opts)
			require.NoError(t, err)
			assert.Equal(t, tt.wantFields, got.Fields)
		})
	}

}

func TestNewResultObjectErrors(t *testing.T) {
	tests := []struct {
		desc string
		give interface{}
		opts resultOptions
		err  string
	}{
		{
			desc: "unexported fields",
			give: struct {
				Out

				writer io.Writer
			}{},
			err: `unexported fields not allowed in dig.Out, did you mean to export "writer" (io.Writer)`,
		},
		{
			desc: "error field",
			give: struct {
				Out

				Error error
			}{},
			err: `bad field "Error" of struct { dig.Out; Error error }: cannot return an error here, return it from the constructor instead`,
		},
		{
			desc: "nested dig.In",
			give: struct {
				Out

				Nested struct{ In }
			}{},
			err: `bad field "Nested"`,
		},
		{
			desc: "group with name should fail",
			give: struct {
				Out

				Foo string `group:"foo" name:"bar"`
			}{},
			err: "cannot use named values with value groups: " +
				`name:"bar" provided with group:"foo"`,
		},
		{
			desc: "group marked as optional",
			give: struct {
				Out

				Foo string `group:"foo" optional:"true"`
			}{},
			err: "value groups cannot be optional",
		},
		{
			desc: "name option",
			give: struct {
				Out

				Reader io.Reader
			}{},
			opts: resultOptions{Name: "foo"},
			err:  `cannot specify a name for result objects`,
		},
		{
			desc: "name option with name tag",
			give: struct {
				Out

				A io.Writer `name:"stream-a"`
				B io.Writer
			}{},
			opts: resultOptions{Name: "stream"},
			err:  `cannot specify a name for result objects`,
		},
		{
			desc: "group tag with name option",
			give: struct {
				Out

				Reader io.Reader
				Writer io.Writer `group:"writers"`
			}{},
			opts: resultOptions{Name: "foo"},
			err:  `cannot specify a name for result objects`,
		},
		{
			desc: "flatten on non-slice",
			give: struct {
				Out

				Writer io.Writer `group:"writers,flatten"`
			}{},
			err: "flatten can be applied to slices only",
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			_, err := newResultObject(reflect.TypeOf(tt.give), tt.opts)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.err)
		})
	}
}

type fakeResultVisit struct {
	Visit                result
	AnnotateWithField    *resultObjectField
	AnnotateWithPosition int
	Return               fakeResultVisits
}

func (fv fakeResultVisit) String() string {
	switch {
	case fv.Visit != nil:
		return fmt.Sprintf("Visit(%#v) -> %v", fv.Visit, fv.Return)
	case fv.AnnotateWithField != nil:
		return fmt.Sprintf("AnnotateWithField(%#v) -> %v", *fv.AnnotateWithField, fv.Return)
	default:
		return fmt.Sprintf("AnnotateWithPosition(%v) -> %v", fv.AnnotateWithPosition, fv.Return)
	}
}

type fakeResultVisits []fakeResultVisit

func (vs fakeResultVisits) Visitor(t *testing.T) resultVisitor {
	return &fakeResultVisitor{t: t, visits: vs}
}

type fakeResultVisitor struct {
	t      *testing.T
	visits fakeResultVisits
}

func (fv *fakeResultVisitor) popNext(call string) fakeResultVisit {
	if len(fv.visits) == 0 {
		fv.t.Fatalf("received unexpected call %v: no more calls were expected", call)
	}

	visit := fv.visits[0]
	fv.visits = fv.visits[1:]
	return visit
}

func (fv *fakeResultVisitor) Visit(r result) resultVisitor {
	v := fv.popNext(fmt.Sprintf("Visit(%#v)", r))
	if !reflect.DeepEqual(r, v.Visit) {
		fv.t.Fatalf("received unexpected call Visit(%#v)\nexpected %v", r, v)
	}
	return &fakeResultVisitor{t: fv.t, visits: v.Return}
}

func (fv *fakeResultVisitor) AnnotateWithField(f resultObjectField) resultVisitor {
	v := fv.popNext(fmt.Sprintf("AnnotateWithField(%#v)", f))
	if v.AnnotateWithField == nil || !reflect.DeepEqual(f, *v.AnnotateWithField) {
		fv.t.Fatalf("received unexpected call AnnotateWithField(%#v)\nexpected %v", f, v)
	}
	return &fakeResultVisitor{t: fv.t, visits: v.Return}
}

func (fv *fakeResultVisitor) AnnotateWithPosition(i int) resultVisitor {
	v := fv.popNext(fmt.Sprintf("AnnotateWithPosition(%v)", i))
	if i != v.AnnotateWithPosition {
		fv.t.Fatalf("received unexpected call AnnotateWithPosition(%v)\nexpected %v", i, v)
	}
	return &fakeResultVisitor{t: fv.t, visits: v.Return}
}

func TestWalkResult(t *testing.T) {
	t.Run("invalid result type", func(t *testing.T) {
		type badResult struct{ result }
		visitor := fakeResultVisits{
			{Visit: badResult{}, Return: fakeResultVisits{}},
		}.Visitor(t)
		assert.Panics(t,
			func() {
				walkResult(badResult{}, visitor)
			})
	})

	t.Run("resultObject ordering", func(t *testing.T) {
		type type1 struct{}
		type type2 struct{}
		type type3 struct{}
		type type4 struct{}

		typ := reflect.TypeOf(struct {
			Out

			T1 type1
			T2 type2

			Nested struct {
				Out

				T3 type3
				T4 type4
			}
		}{})

		ro, err := newResultObject(typ, resultOptions{})
		require.NoError(t, err)

		v := fakeResultVisits{
			{
				Visit: ro,
				Return: fakeResultVisits{
					{
						AnnotateWithField: &ro.Fields[0],
						Return: fakeResultVisits{
							{Visit: ro.Fields[0].Result},
						},
					},
					{
						AnnotateWithField: &ro.Fields[1],
						Return: fakeResultVisits{
							{Visit: ro.Fields[1].Result},
						},
					},
					{
						AnnotateWithField: &ro.Fields[2],
						Return: fakeResultVisits{
							{
								Visit: ro.Fields[2].Result,
								Return: fakeResultVisits{
									{
										AnnotateWithField: &ro.Fields[2].Result.(resultObject).Fields[0],
										Return: fakeResultVisits{
											{Visit: ro.Fields[2].Result.(resultObject).Fields[0].Result},
										},
									},
									{
										AnnotateWithField: &ro.Fields[2].Result.(resultObject).Fields[1],
										Return: fakeResultVisits{
											{Visit: ro.Fields[2].Result.(resultObject).Fields[1].Result},
										},
									},
								},
							},
						},
					},
				},
			},
		}.Visitor(t)

		walkResult(ro, v)
	})
}
