# ReflectX

The `ReflectX` package aims to make developers' life easier, inspired
by [reflectutils](https://github.com/muir/reflectutils), [reflections](https://github.com/oleiade/reflections)
and [gpath](https://github.com/tenntenn/gpath).

## Install

```shell
go get github.com/black-06/reflectx
```

## Usage

### GetValue

It returns the content of given value by a path, the path is represented by Go's expression which can be parsed by
`go/parser.ParseExpr`.

It can access deeper levels of values
than [reflections.GetField](https://github.com/oleiade/reflections/blob/master/README.md#getfield), more types of values
than [gpath.At](https://github.com/tenntenn/gpath)

```go
type (
	Foo struct {
		Names []string
	}
	Bar struct {
		Foo
	}
)

value := Bar{Foo{Names: []string{"a", "b", "c"}}}
// For the value, you can also choose &value1.
// For the path, you can also choose 
// - "Names[1]", because Foo is embed struct
// - "$.Foo.Names[1]", '$' means the root path of the value
// - "$.Names[1]"
v, err := GetValue(value, "Foo.Names[1]") // result v is "b"
v, err = GetValue("bar", "$")             // result v is "bar"
```

### SetValue

It sets the content of given value by a path. Note: the value must be addressable, in general, it is a pointer.

```go
value := Bar{Foo{Names: []string{"some", "string"}}}

err := SetValue(&value, "Foo.Names[1]", "value")     // value is Bar{Foo{Names: []string{"some", "value"}}}
err = SetValue(&value, "Foo.Names[1][0]", byte('m')) // value is Bar{Foo{Names: []string{"some", "malue"}}}
```

### ValueEntryByPath

In fact, both `GetValue` and `SetValue` calls `ValueEntryByPath`. It returns a `ValueEntry` of given value by a path.
The `ValueEntry` contains many useful values such as the actual `reflect.Value`.

For example, we can get tag in `StructField`:

```go
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

field.Tag.Get("json") // "num"
```

