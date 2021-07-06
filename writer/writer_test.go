package writer_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"testing"

	"github.com/knightso/json-partial-streaming/writer"
)

func TestWrite(t *testing.T) {
	buf := new(bytes.Buffer)
	w := writer.New(buf)

	type Child struct {
		Name        string
		Values      *writer.Value
		ArrayValues *writer.Value
	}

	type Parent struct {
		Name              string
		Value             *writer.Value
		Values            *writer.Value
		NumberArrayValues *writer.Value
		StringArrayValues *writer.Value
		StructArrayValues *writer.Value
		Children          []*Child
		Quoted            string
		Tagged            string `json:"tagged"`
		EmptyKey          *writer.Value
	}

	type StructValue struct {
		Hoge string
		Fuga int
	}

	pv, err := w.NewValue("$.Value", func(w io.Writer) error {
		jsn, err := json.Marshal(&StructValue{
			Hoge: "hoge1",
			Fuga: 1,
		})
		if err != nil {
			return err
		}
		if _, err := w.Write(jsn); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	p := &Parent{
		Name:  "parent",
		Value: pv,
		Values: w.MustNewValue("$.Values", func(w io.Writer) error {
			if _, err := w.Write([]byte("[")); err != nil {
				return err
			}
			for i := 0; i < 3; i++ {
				if i > 0 {
					if _, err := w.Write([]byte(",")); err != nil {
						return err
					}
				}
				jsn, err := json.Marshal(&StructValue{
					Hoge: fmt.Sprintf("hoge%d", i+1),
					Fuga: i + 1,
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
		}),
		NumberArrayValues: w.MustNewArrayValue("$.NumberArrayValues", func(w writer.ElementWriter) error {
			for i := 0; i < 3; i++ {
				if err := w.WriteElement(i); err != nil {
					return err
				}
			}

			return nil
		}),
		StringArrayValues: w.MustNewArrayValue("$.StringArrayValues", func(w writer.ElementWriter) error {
			for i := 0; i < 3; i++ {
				if err := w.WriteElement(fmt.Sprintf("test%d", i)); err != nil {
					return err
				}
			}
			return nil
		}),
		StructArrayValues: w.MustNewArrayValue("$.StructArrayValues", func(w writer.ElementWriter) error {
			for i := 0; i < 3; i++ {
				sv := &StructValue{
					Hoge: fmt.Sprintf("hoge%d", i+4),
					Fuga: i + 4,
				}
				if err := w.WriteElement(sv); err != nil {
					return err
				}
			}

			return nil
		}),
		Quoted: `"quoted test"`,
		Tagged: `"tagged test"`,
		EmptyKey: w.MustNewValue("", func(w io.Writer) error {
			fmt.Fprintf(w, `"empty key test"`)
			return nil
		}),
	}

	for i := 0; i < 3; i++ {
		i := i
		c := &Child{
			Name: fmt.Sprintf(`child%d`, i),
			Values: w.MustNewValue(fmt.Sprintf("$.Child[%d].Values", i), func(w io.Writer) error {
				if _, err := w.Write([]byte("[")); err != nil {
					return err
				}
				for j := 0; j < 3; j++ {
					if j > 0 {
						if _, err := w.Write([]byte(",")); err != nil {
							return err
						}
					}
					jsn, err := json.Marshal(&StructValue{
						Hoge: fmt.Sprintf("hoge%d-%d", i, j),
						Fuga: i*10 + j,
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
			}),
			ArrayValues: w.MustNewArrayValue(fmt.Sprintf("$.Child[%d].ArrayValues", i), func(w writer.ElementWriter) error {
				for j := 0; j < 3; j++ {
					sv := &StructValue{
						Hoge: fmt.Sprintf("hoge%d-%d", i, j+3),
						Fuga: i*10 + (j + 3),
					}
					if err := w.WriteElement(sv); err != nil {
						return err
					}
				}

				return nil
			}),
		}
		p.Children = append(p.Children, c)
	}

	encoder := json.NewEncoder(w)

	if err := encoder.Encode(p); err != nil {
		fmt.Println(err.Error())
		return
	}

	d, err := ioutil.ReadFile("testdata/write_expected.json")
	if err != nil {
		t.Fatal(err)
	}
	if expected, result := string(d), buf.String(); result != expected {
		t.Fatalf("result expected:%s, but was %s", expected, result)
	}

	// unmarshal again
	m := map[string]interface{}{}
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	//t.Log(m)
}

func TestMarshalJSON(t *testing.T) {
	w := writer.New(ioutil.Discard)

	v := w.MustNewValue("testkey", func(w io.Writer) error {
		return nil
	})

	b, err := v.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}

	if expected, actual := `"\\🎏testkey"`, string(b); expected != actual {
		t.Errorf("MarshalJSON failed. expected %s but was %s", expected, actual)
	}
}

func TestMarshalJSONEscaping(t *testing.T) {
	w := writer.New(ioutil.Discard)

	v := w.MustNewValue(`\test"key"`, func(w io.Writer) error {
		return nil
	})

	b, err := v.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}

	if expected, actual := `"\\🎏\\test\"key\""`, string(b); expected != actual {
		t.Errorf("MarshalJSON failed. expected %s but was %s", expected, actual)
	}
}
