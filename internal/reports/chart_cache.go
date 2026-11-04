package reports

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

var chartReferences = regexp.MustCompile(`(?s)<(numRef|strRef)><f>.*?</f></(?:numRef|strRef)>`)

// Excelize writes native cell references without cached labels/points. Populate
// the optional OOXML caches so preview readers show the same chart immediately,
// while keeping the cell references editable in Excel. Other ZIP entries are
// copied compressed, avoiding a second compression pass over the user register.
func chartCaches(data []byte, workbook *excelize.File) ([]byte, error) {
	input, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	var output bytes.Buffer
	writer := zip.NewWriter(&output)
	for _, entry := range input.File {
		if !strings.HasPrefix(entry.Name, "xl/charts/chart") || !strings.HasSuffix(entry.Name, ".xml") {
			if err = writer.Copy(entry); err != nil {
				return nil, err
			}
			continue
		}
		reader, e := entry.Open()
		if e != nil {
			return nil, e
		}
		raw, e := io.ReadAll(reader)
		_ = reader.Close()
		if e != nil {
			return nil, e
		}
		var cacheErr error
		raw = chartReferences.ReplaceAllFunc(raw, func(reference []byte) []byte {
			if cacheErr != nil {
				return reference
			}
			var result []byte
			result, cacheErr = cacheReference(reference, workbook)
			return result
		})
		if cacheErr != nil {
			return nil, cacheErr
		}
		header := entry.FileHeader
		part, e := writer.CreateHeader(&header)
		if e != nil {
			return nil, e
		}
		if _, e = part.Write(raw); e != nil {
			return nil, e
		}
	}
	if err = writer.Close(); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func cacheReference(reference []byte, workbook *excelize.File) ([]byte, error) {
	var ref struct {
		XMLName xml.Name
		Formula string `xml:"f"`
	}
	if err := xml.Unmarshal(reference, &ref); err != nil {
		return nil, err
	}
	separator := strings.LastIndex(ref.Formula, "!")
	if separator < 1 {
		return nil, fmt.Errorf("invalid chart range")
	}
	sheet := strings.ReplaceAll(strings.Trim(ref.Formula[:separator], "'"), "''", "'")
	address := strings.ReplaceAll(ref.Formula[separator+1:], "$", "")
	bounds := strings.Split(address, ":")
	firstCol, firstRow, err := excelize.CellNameToCoordinates(bounds[0])
	if err != nil {
		return nil, err
	}
	lastCol, lastRow := firstCol, firstRow
	if len(bounds) > 1 {
		lastCol, lastRow, err = excelize.CellNameToCoordinates(bounds[1])
		if err != nil {
			return nil, err
		}
	}
	count := (lastCol - firstCol + 1) * (lastRow - firstRow + 1)
	var cache bytes.Buffer
	numeric := ref.XMLName.Local == "numRef"
	tag := "strCache"
	if numeric {
		tag = "numCache"
	}
	fmt.Fprintf(&cache, "<%s>", tag)
	if numeric {
		cache.WriteString("<formatCode>General</formatCode>")
	}
	fmt.Fprintf(&cache, `<ptCount val="%d"/>`, count)
	index := 0
	for row := firstRow; row <= lastRow; row++ {
		for col := firstCol; col <= lastCol; col++ {
			value, e := workbook.GetCellValue(sheet, cell(col, row), excelize.Options{RawCellValue: numeric})
			if e != nil {
				return nil, e
			}
			if value != "" {
				if numeric {
					if _, e = strconv.ParseFloat(value, 64); e != nil {
						return nil, fmt.Errorf("non-numeric chart value: %w", e)
					}
				}
				fmt.Fprintf(&cache, `<pt idx="%d"><v>`, index)
				if e = xml.EscapeText(&cache, []byte(value)); e != nil {
					return nil, e
				}
				cache.WriteString("</v></pt>")
			}
			index++
		}
	}
	fmt.Fprintf(&cache, "</%s></%s>", tag, ref.XMLName.Local)
	closing := []byte("</" + ref.XMLName.Local + ">")
	return append(append([]byte(nil), bytes.TrimSuffix(reference, closing)...), cache.Bytes()...), nil
}
