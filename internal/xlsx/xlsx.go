// Package xlsx writes small Excel (.xlsx) workbooks with no dependency: an
// xlsx file is a zip of a few XML parts. It covers what reports need —
// several sheets, right-to-left layout, column widths, a frozen header row,
// an auto-filter and a handful of cell styles — and nothing more.
package xlsx

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Style is one of the built-in cell styles.
type Style int

const (
	Plain  Style = iota
	Header       // bold white on brand green, centered
	Title        // large bold
	Bold         //
	Good         // green fill (a correct answer)
	Bad          // red fill (a wrong answer)
	Muted        // grey text (time ran out / not answered)
	Gold         // podium highlight
)

// Cell is a styled value. A row may also hold bare values (string, int,
// int64, float64), which get the Plain style.
type Cell struct {
	V any
	S Style
}

// Sheet is one worksheet.
type Sheet struct {
	Name   string    // at most 31 characters, no []:*?/\
	Widths []float64 // column widths in characters; missing columns use Excel's default
	Rows   [][]any
	RTL    bool
	// HeaderRow (1-based) is frozen and gets an auto-filter over the rows
	// below it; 0 means none.
	HeaderRow int
}

// Write encodes the workbook to w.
func Write(w io.Writer, sheets []Sheet) error {
	z := zip.NewWriter(w)
	put := func(name, body string) error {
		f, err := z.Create(name)
		if err != nil {
			return err
		}
		_, err = io.WriteString(f, body)
		return err
	}

	var ct, wbSheets, wbRels strings.Builder
	ct.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
		`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
		`<Default Extension="xml" ContentType="application/xml"/>` +
		`<Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>` +
		`<Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/>`)
	for i, sh := range sheets {
		n := i + 1
		fmt.Fprintf(&ct, `<Override PartName="/xl/worksheets/sheet%d.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>`, n)
		fmt.Fprintf(&wbSheets, `<sheet name="%s" sheetId="%d" r:id="rId%d"/>`, esc(sheetName(sh.Name, n)), n, n)
		fmt.Fprintf(&wbRels, `<Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet%d.xml"/>`, n, n)
	}
	ct.WriteString(`</Types>`)
	stylesID := len(sheets) + 1
	fmt.Fprintf(&wbRels, `<Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>`, stylesID)

	parts := []struct{ name, body string }{
		{"[Content_Types].xml", ct.String()},
		{"_rels/.rels", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
			`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>` +
			`</Relationships>`},
		{"xl/workbook.xml", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">` +
			`<sheets>` + wbSheets.String() + `</sheets></workbook>`},
		{"xl/_rels/workbook.xml.rels", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
			wbRels.String() + `</Relationships>`},
		{"xl/styles.xml", stylesXML},
	}
	for _, p := range parts {
		if err := put(p.name, p.body); err != nil {
			return err
		}
	}
	for i, sh := range sheets {
		if err := put(fmt.Sprintf("xl/worksheets/sheet%d.xml", i+1), sheetXML(sh)); err != nil {
			return err
		}
	}
	return z.Close()
}

// Bytes is Write into memory.
func Bytes(sheets []Sheet) ([]byte, error) {
	var b bytes.Buffer
	err := Write(&b, sheets)
	return b.Bytes(), err
}

func sheetXML(sh Sheet) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">`)
	b.WriteString(`<sheetViews><sheetView workbookViewId="0"`)
	if sh.RTL {
		b.WriteString(` rightToLeft="1"`)
	}
	if sh.HeaderRow > 0 {
		fmt.Fprintf(&b, `><pane ySplit="%d" topLeftCell="A%d" activePane="bottomLeft" state="frozen"/></sheetView>`, sh.HeaderRow, sh.HeaderRow+1)
	} else {
		b.WriteString(`/>`)
	}
	b.WriteString(`</sheetViews>`)
	if len(sh.Widths) > 0 {
		b.WriteString(`<cols>`)
		for i, w := range sh.Widths {
			fmt.Fprintf(&b, `<col min="%d" max="%d" width="%.1f" customWidth="1"/>`, i+1, i+1, w)
		}
		b.WriteString(`</cols>`)
	}
	b.WriteString(`<sheetData>`)
	maxCols := 0
	for r, row := range sh.Rows {
		if len(row) > maxCols {
			maxCols = len(row)
		}
		fmt.Fprintf(&b, `<row r="%d">`, r+1)
		for c, v := range row {
			writeCell(&b, ColName(c)+strconv.Itoa(r+1), v)
		}
		b.WriteString(`</row>`)
	}
	b.WriteString(`</sheetData>`)
	if sh.HeaderRow > 0 && len(sh.Rows) > sh.HeaderRow && maxCols > 0 {
		fmt.Fprintf(&b, `<autoFilter ref="A%d:%s%d"/>`, sh.HeaderRow, ColName(maxCols-1), len(sh.Rows))
	}
	b.WriteString(`</worksheet>`)
	return b.String()
}

func writeCell(b *strings.Builder, ref string, v any) {
	st := Plain
	if c, ok := v.(Cell); ok {
		v, st = c.V, c.S
	}
	sAttr := ""
	if st != Plain {
		sAttr = fmt.Sprintf(` s="%d"`, st)
	}
	switch x := v.(type) {
	case nil:
		if st != Plain {
			fmt.Fprintf(b, `<c r="%s"%s/>`, ref, sAttr)
		}
	case int:
		fmt.Fprintf(b, `<c r="%s"%s><v>%d</v></c>`, ref, sAttr, x)
	case int64:
		fmt.Fprintf(b, `<c r="%s"%s><v>%d</v></c>`, ref, sAttr, x)
	case float64:
		fmt.Fprintf(b, `<c r="%s"%s><v>%s</v></c>`, ref, sAttr, strconv.FormatFloat(x, 'f', -1, 64))
	default:
		s := fmt.Sprint(x)
		fmt.Fprintf(b, `<c r="%s"%s t="inlineStr"><is><t xml:space="preserve">%s</t></is></c>`, ref, sAttr, esc(s))
	}
}

// ColName turns a 0-based column index into its letters (0 → A, 26 → AA).
func ColName(i int) string {
	s := ""
	for i++; i > 0; i = (i - 1) / 26 {
		s = string(rune('A'+(i-1)%26)) + s
	}
	return s
}

// esc XML-escapes s and drops the control characters XML 1.0 forbids.
func esc(s string) string {
	s = strings.Map(func(r rune) rune {
		if r < 0x20 && r != '\t' && r != '\n' && r != '\r' {
			return -1
		}
		return r
	}, s)
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

func sheetName(name string, n int) string {
	name = strings.Map(func(r rune) rune {
		if strings.ContainsRune(`[]:*?/\`, r) {
			return '-'
		}
		return r
	}, strings.TrimSpace(name))
	if r := []rune(name); len(r) > 31 {
		name = string(r[:31])
	}
	if name == "" {
		name = "Sheet" + strconv.Itoa(n)
	}
	return name
}

// stylesXML: cellXfs indexes match the Style constants.
const stylesXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
	`<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">` +
	`<fonts count="5">` +
	`<font><sz val="11"/><name val="Tahoma"/></font>` +
	`<font><b/><sz val="11"/><color rgb="FFFFFFFF"/><name val="Tahoma"/></font>` +
	`<font><b/><sz val="15"/><color rgb="FF006839"/><name val="Tahoma"/></font>` +
	`<font><b/><sz val="11"/><name val="Tahoma"/></font>` +
	`<font><sz val="11"/><color rgb="FF8A8A8A"/><name val="Tahoma"/></font>` +
	`</fonts>` +
	`<fills count="6">` +
	`<fill><patternFill patternType="none"/></fill>` +
	`<fill><patternFill patternType="gray125"/></fill>` +
	`<fill><patternFill patternType="solid"><fgColor rgb="FF006839"/><bgColor indexed="64"/></patternFill></fill>` +
	`<fill><patternFill patternType="solid"><fgColor rgb="FFDDF3E4"/><bgColor indexed="64"/></patternFill></fill>` +
	`<fill><patternFill patternType="solid"><fgColor rgb="FFFBE0E0"/><bgColor indexed="64"/></patternFill></fill>` +
	`<fill><patternFill patternType="solid"><fgColor rgb="FFFFF1C9"/><bgColor indexed="64"/></patternFill></fill>` +
	`</fills>` +
	`<borders count="2">` +
	`<border><left/><right/><top/><bottom/><diagonal/></border>` +
	`<border><left style="thin"><color rgb="FFD0D7DE"/></left><right style="thin"><color rgb="FFD0D7DE"/></right>` +
	`<top style="thin"><color rgb="FFD0D7DE"/></top><bottom style="thin"><color rgb="FFD0D7DE"/></bottom><diagonal/></border>` +
	`</borders>` +
	`<cellStyleXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/></cellStyleXfs>` +
	`<cellXfs count="8">` +
	`<xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0"><alignment vertical="center" wrapText="1"/></xf>` +
	`<xf numFmtId="0" fontId="1" fillId="2" borderId="1" xfId="0" applyFont="1" applyFill="1" applyBorder="1"><alignment horizontal="center" vertical="center" wrapText="1"/></xf>` +
	`<xf numFmtId="0" fontId="2" fillId="0" borderId="0" xfId="0" applyFont="1"/>` +
	`<xf numFmtId="0" fontId="3" fillId="0" borderId="0" xfId="0" applyFont="1"><alignment vertical="center" wrapText="1"/></xf>` +
	`<xf numFmtId="0" fontId="0" fillId="3" borderId="1" xfId="0" applyFill="1" applyBorder="1"><alignment vertical="center" wrapText="1"/></xf>` +
	`<xf numFmtId="0" fontId="0" fillId="4" borderId="1" xfId="0" applyFill="1" applyBorder="1"><alignment vertical="center" wrapText="1"/></xf>` +
	`<xf numFmtId="0" fontId="4" fillId="0" borderId="0" xfId="0" applyFont="1"><alignment vertical="center" wrapText="1"/></xf>` +
	`<xf numFmtId="0" fontId="3" fillId="5" borderId="1" xfId="0" applyFont="1" applyFill="1" applyBorder="1"><alignment vertical="center" wrapText="1"/></xf>` +
	`</cellXfs>` +
	`<cellStyles count="1"><cellStyle name="Normal" xfId="0" builtinId="0"/></cellStyles>` +
	`</styleSheet>`
