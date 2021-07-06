# json-partial-streaming
JSON partial streaming writer🎏

## What's it for?

Package `encoding/json` needs large memory to encode or marshal objects which have huge size of array in it.

e.g.
```
{
  "name": "parent",
  "children": [
	{"name": "child1", "age", 1},
	{"name": "child2", "age", 2},

  ... omit ...

	{"name": "child1000000", "age", 1000000},
  ]
}
```

`json-partial-streaming` writer can use with `encoding/json` Encoder and gives you control of large JSON values  streaming encoding.

## Usage

**Import**

```go
import "github.com/knightso/json-partial-streaming/writer"
```

**Define `*writer.Value` field to your struct.**

```go
type Root struct {
  Name        string
  Children      *writer.Value
}
```

**Initialize `Writer`.**

```go
w := writer.New(writerToOutput)
```

**Prepare struct.**

```go
// prepare large Value
// key can be any string(even empty), but it must be unique.
pv, err := w.NewValue("Children", func(w io.Writer) error {
  // you can handle their encoding by yourself.
  if _, err := w.Write([]byte("[")); err != nil {
    return err
  }
  for i := 0; i < 1000000; i++ {
    if i > 0 {
      if _, err := w.Write([]byte(",")); err != nil {
        return err
      }
    }
    jsn, err := json.Marshal(&Child{
      Name: fmt.Sprintf("child%d", i+1),
      Age: i + 1,
    })
    if err != nil {
      return err
    }
    if _, err := w.Write(jsn); err != nil {
      return err
    }
  }
  if _, err := w.Write([]byte("]")); err != nil {
    return err
  }
  return nil
})
if err != nil {
  return err
}

// prepare struct
parent := &Parent{
  Name: "parent",
  Value: pv,
}
```

you can also use helper shorthand methods.

```go
parent := &Parent{
  Name: "parent",
  Value: writer.MustNewArrayValue("", func(w writer.ElementWriter) error {
  for j := 0; j < 1000000; j++ {
    if err := w.WriteElement(&Child{
      Name: fmt.Sprintf("child%d", i+1),
      Age: i + 1,
    }); err != nil {
      return err
    }
  }
  return nil
}),

}
```

look at the code and the tests for other usages.

**Encode!!**

```go
if err := json.NewEncoder(w).Encode(p); err != nil {
  return error
}
```

## Restriction

- You cannot put the reserved prefix `\🎏` to the string key or value.
(TODO: Add an option to change prefix)
