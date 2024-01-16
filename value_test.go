package reflectx

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetTagByEntry(t *testing.T) {
	type Data struct {
		ID     string `json:"id"`
		Record struct {
			Num int `json:"num"`
		}
	}

	entry, err := ValueEntryByPath(Data{}, "Record.Num")
	assert.Empty(t, err)

	field, ok := entry.StructField()
	assert.True(t, ok)
	assert.Equal(t, "num", field.Tag.Get("json"))
}

type (
	Foo struct {
		Name1 string
		Name2 *string
	}
	Bar struct {
		Foo
		Foo2 *Foo
		Any  any
	}
	Combine struct {
		BarMap map[string]Bar
		Names  []*Bar
	}

	Ptr1 struct{ ID *int }
	Ptr2 struct{ *Ptr1 }
	Ptr3 struct{ PtrSlice []*Ptr2 }
	Ptr4 struct{ PtrMap map[string]*Ptr3 }
	Ptr5 struct{ Ptr ****Ptr1 }
)

func Ref[T any](value T) *T {
	return &value
}

func TestGetValue(t *testing.T) {
	tests := []struct {
		name     string
		value    any
		paths    []string
		err      string
		expected any
	}{
		{name: "combine 1", value: Combine{BarMap: map[string]Bar{"b": {Foo: Foo{Name1: "hello"}}}}, paths: []string{`BarMap["b"].Foo.Name1`}, expected: "hello"},
		{name: "combine 2", value: Combine{Names: []*Bar{nil, {Foo2: &Foo{Name2: Ref("world")}}}}, paths: []string{`Names[1].Foo2.Name2`}, expected: "world"},

		{name: "u8", value: uint8(3), paths: []string{"$"}, expected: uint8(3)},
		{name: "i16", value: int16(3), paths: []string{"$"}, expected: int16(3)},
		{name: "f32", value: float32(3.1415926), paths: []string{"$"}, expected: float32(3.1415926)},
		{name: "f64", value: 3.1415926, paths: []string{"$"}, expected: 3.1415926},
		{name: "string 1", value: "foo bar", paths: []string{"$"}, expected: "foo bar"},
		{name: "string 2", value: "foo bar", paths: []string{"$[4]"}, expected: byte('b')},
		{name: "array", value: [3]byte{'a', 'v', 'b'}, paths: []string{"$[1]"}, expected: byte('v')},
		{name: "slice", value: []byte{'a', 'v', 'b'}, paths: []string{"$[1]"}, expected: byte('v')},
		{name: "out of range", value: []byte("hello"), paths: []string{"$[10]"}, err: "index 10 out of range 5"},

		{name: "map", value: map[string]int32{"a": 1, "b": 2}, paths: []string{`$["a"]`}, expected: int32(1)},
		{name: "i key map", value: map[int]int32{1: 1, 2: 2}, paths: []string{`$[1]`}, expected: int32(1)},
		{name: "f key map", value: map[float32]int32{1.1: 1, 2.2: 2}, paths: []string{`$[1.1]`}, expected: int32(1)},
		{name: "invalid key", value: map[int]int32{1: 1, 2: 2}, paths: []string{`$["1"]`}, err: `invalid map key "1"`},
		{name: "key not found 1", value: map[string]int32{"a": 1, "b": 2}, paths: []string{`$["c"]`}, expected: int32(0)},
		{name: "key not found 2", value: map[string][]string{"a": {}, "b": {}}, paths: []string{`$["c"]`}, expected: []string(nil)},

		{name: "struct 1", value: Foo{Name1: "bar1"}, paths: []string{"Name1", "$.Name1"}, expected: "bar1"},
		{name: "struct 2", value: Foo{Name2: Ref("bar2")}, paths: []string{"Name2", "$.Name2"}, expected: "bar2"},
		{name: "struct 3", value: Foo{}, paths: []string{"Name2", "$.Name2"}, expected: ""},
		{name: "embed struct", value: Bar{Foo: Foo{Name1: "wow"}}, paths: []string{"$.Name1", "Foo.Name1", "$.Foo.Name1"}, expected: "wow"},
		{name: "ignore nil", value: Bar{}, paths: []string{"Foo2.Name1", "$.Foo2.Name1"}, expected: ""},
		{name: "unexported field", value: bytes.NewBuffer([]byte("hello")), paths: []string{"buf[0]"}, err: "cannot access unexported field"},
		{name: "any field", value: Bar{Any: map[string][]int{"k1": {0, 1, 2}}}, paths: []string{`Any["k1"][1]`}, expected: 1},

		{name: "ptr 1", value: &Ptr4{PtrMap: map[string]*Ptr3{"p": {PtrSlice: []*Ptr2{{Ptr1: &Ptr1{ID: Ref(10)}}}}}}, paths: []string{`PtrMap["p"].PtrSlice[0].ID`}, expected: 10},
		{name: "ptr 2", value: &Bar{Any: Ref(Ref(Ref(Ptr1{ID: Ref(3)})))}, paths: []string{`Any.ID`}, expected: 3},
		{name: "ptr 3", value: &Ptr5{}, paths: []string{`Ptr.ID`}, expected: 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for _, path := range test.paths {
				actual, err := GetValue(test.value, path)
				if test.err != "" {
					assert.EqualError(t, err, test.err)
				} else {
					assert.Empty(t, err)
					assert.Equal(t, test.expected, actual)
				}
			}
		})
	}
}

func TestDeRef(t *testing.T) {
	var p *struct{} = nil
	pp := &p
	rv := reflect.ValueOf(pp)
	rv = defaultValueOption.DeRef(rv)
	assert.True(t, rv.CanInterface())
	assert.Equal(t, rv.Interface(), struct{}{})
	assert.Equal(t, **pp, struct{}{})
}

func TestSetValue(t *testing.T) {
	tests := []struct {
		name     string
		value    any
		path     string
		v        any
		err      string
		expected any
	}{
		{name: "u8", value: Ref(uint8(3)), path: "$", v: uint8(10), expected: Ref(uint8(10))},
		{name: "i16", value: Ref(int16(3)), path: "$", v: int16(10), expected: Ref(int16(10))},
		{name: "f32", value: Ref(float32(3.1415926)), path: "$", v: float32(2.71828), expected: Ref(float32(2.71828))},
		{name: "f64", value: Ref(3.1415926), path: "$", v: 2.71828, expected: Ref(2.71828)},
		{name: "string 1", value: Ref("foo bar"), path: "$", v: "bar foo", expected: Ref("bar foo")},
		{name: "string 2", value: Ref("foo bar"), path: "$[4]", v: byte('t'), expected: Ref("foo tar")},
		{name: "string 3", value: Ref("foo 谁"), path: "$[4]", v: '我', expected: Ref("foo 我")},
		{name: "array", value: &[3]byte{'a', 'v', 'b'}, path: "$[1]", v: byte('c'), expected: &[3]byte{'a', 'c', 'b'}},
		{name: "slice", value: &[]byte{'a', 'v', 'b'}, path: "$[1]", v: byte('c'), expected: &[]byte{'a', 'c', 'b'}},
		{name: "out of range", value: &[]byte{'h', 'e', 'l', 'l', 'o'}, path: "$[10]", err: "index 10 out of range 5"},

		{name: "map", value: &map[string]int{"a": 1, "b": 2}, path: `$["a"]`, v: 3, expected: &map[string]int{"a": 3, "b": 2}},
		{name: "byte key map", value: &map[byte]int{'a': 1, 'b': 2}, path: `$['c']`, v: 3, expected: &map[byte]int{'a': 1, 'b': 2, 'c': 3}},
		{name: "complex key map", value: &map[complex64]int{1i: 1, 2i: 2}, path: `$[3i]`, v: 3, expected: &map[complex64]int{1i: 1, 2i: 2, 3i: 3}},
		{name: "f key map", value: &map[float32]int{1.1: 1, 2.2: 2}, path: `$[1.1]`, v: 3, expected: &map[float32]int{1.1: 3, 2.2: 2}},
		{name: "invalid key", value: &map[int]int{1: 1, 2: 2}, path: `$["1"]`, err: `invalid map key "1"`},
		{name: "add key 1", value: &map[string]int{"a": 1, "b": 2}, path: `$["c"]`, v: 3, expected: &map[string]int{"a": 1, "b": 2, "c": 3}},
		{name: "add key 2", value: &map[string][]string{"a": {}, "b": {}}, path: `$["c"]`, v: []string{"ccc"}, expected: &map[string][]string{"a": {}, "b": {}, "c": {"ccc"}}},

		{name: "struct 1", value: &Foo{Name1: "bar1"}, v: "bar12", path: "Name1", expected: &Foo{Name1: "bar12"}},
		{name: "struct 2", value: &Foo{Name2: Ref("bar2")}, path: "Name2", v: "bar22", expected: &Foo{Name2: Ref("bar22")}},
		{name: "struct 3", value: &Foo{}, path: "Name2", v: "bar3", expected: &Foo{Name2: Ref("bar3")}},
		{name: "embed struct", value: &Bar{Foo: Foo{Name1: "wow"}}, path: "$.Name1", v: "mom", expected: &Bar{Foo: Foo{Name1: "mom"}}},
		{name: "unexported field", value: bytes.NewBuffer([]byte("hello")), path: "buf[0]", v: 'b', err: "cannot access unexported field"},
		{name: "any field 1", value: &Bar{Any: map[string][]int{"k1": {0, 1, 2}}}, path: `Any["k1"][1]`, v: 11, expected: &Bar{Any: map[string][]int{"k1": {0, 11, 2}}}},
		{name: "any field 2", value: &Bar{Any: map[string][]int{"k1": {0, 1, 2}}}, path: `Any["k2"]`, v: []int{3}, expected: &Bar{Any: map[string][]int{"k1": {0, 1, 2}, "k2": {3}}}},
		{name: "any field 3", value: &Bar{Any: &map[string]string{"k1": "v1"}}, path: `Any["k2"]`, v: "v2", expected: &Bar{Any: &map[string]string{"k1": "v1", "k2": "v2"}}},

		{name: "ptr 1", value: &Ptr4{PtrMap: map[string]*Ptr3{"p": {PtrSlice: []*Ptr2{{Ptr1: &Ptr1{ID: Ref(10)}}}}}}, path: `PtrMap["p"].PtrSlice[0].ID`, v: 9, expected: &Ptr4{PtrMap: map[string]*Ptr3{"p": {PtrSlice: []*Ptr2{{Ptr1: &Ptr1{ID: Ref(9)}}}}}}},
		{name: "ptr 2", value: &Bar{Any: Ref(Ref(Ref(Ptr1{ID: Ref(3)})))}, path: `Any.ID`, v: 6, expected: &Bar{Any: Ref(Ref(Ref(Ptr1{ID: Ref(6)})))}},
		{name: "ptr 3", value: &Ptr5{}, path: `Ptr.ID`, v: 69, expected: &Ptr5{Ptr: Ref(Ref(Ref(Ref(Ptr1{ID: Ref(69)}))))}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := SetValue(test.value, test.path, test.v)
			if test.err != "" {
				assert.EqualError(t, err, test.err)
			} else {
				assert.Empty(t, err)
				assert.Equal(t, test.expected, test.value)
			}
		})
	}
}
