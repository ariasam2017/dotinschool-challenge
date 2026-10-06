package xlsx

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"strings"
	"testing"
)

func TestColName(t *testing.T) {
	for i, want := range map[int]string{0: "A", 25: "Z", 26: "AA", 27: "AB", 701: "ZZ", 702: "AAA"} {
		if got := ColName(i); got != want {
			t.Errorf("ColName(%d) = %s, want %s", i, got, want)
		}
	}
}

// Every part is well-formed XML and the values (escaped) land in the sheet.
func TestWriteIsValid(t *testing.T) {
	data, err := Bytes([]Sheet{
		{Name: "خلاصه", RTL: true, Rows: [][]any{{Cell{V: "عنوان", S: Title}}, {"a<b & \"c\"", 42, int64(7), 1.5, nil}}},
		{Name: "bad/name:[x]", HeaderRow: 1, Widths: []float64{10}, Rows: [][]any{{Cell{V: "h", S: Header}}, {"ctl\x01char"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	z, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("not a zip: %v", err)
	}
	parts := map[string]string{}
	for _, f := range z.File {
		r, _ := f.Open()
		b, _ := io.ReadAll(r)
		r.Close()
		parts[f.Name] = string(b)
		d := xml.NewDecoder(bytes.NewReader(b))
		for {
			if _, err := d.Token(); err == io.EOF {
				break
			} else if err != nil {
				t.Fatalf("%s is not well-formed: %v", f.Name, err)
			}
		}
	}
	for _, name := range []string{"[Content_Types].xml", "_rels/.rels", "xl/workbook.xml", "xl/_rels/workbook.xml.rels",
		"xl/styles.xml", "xl/worksheets/sheet1.xml", "xl/worksheets/sheet2.xml"} {
		if _, ok := parts[name]; !ok {
			t.Fatalf("missing part %s", name)
		}
	}
	s1 := parts["xl/worksheets/sheet1.xml"]
	for _, want := range []string{`rightToLeft="1"`, "a&lt;b &amp;", "<v>42</v>", "<v>1.5</v>", "عنوان"} {
		if !strings.Contains(s1, want) {
			t.Errorf("sheet1 lacks %q", want)
		}
	}
	s2 := parts["xl/worksheets/sheet2.xml"]
	if !strings.Contains(s2, `state="frozen"`) || !strings.Contains(s2, `<autoFilter ref="A1:A2"/>`) || strings.Contains(s2, "\x01") {
		t.Errorf("sheet2 header/filter/escaping wrong: %s", s2)
	}
	if !strings.Contains(parts["xl/workbook.xml"], `name="bad-name--x-"`) {
		t.Errorf("sheet name not sanitized: %s", parts["xl/workbook.xml"])
	}
}
