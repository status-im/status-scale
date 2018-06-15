package metrics

import (
	"fmt"
	"io"

	"github.com/olekukonko/tablewriter"
)

type Stringer interface {
	String() string
}

func ToASCII(tab *Table, writer io.Writer) *tablewriter.Table {
	atab := tablewriter.NewWriter(writer)
	// markdown format
	atab.SetBorders(tablewriter.Border{Left: true, Top: false, Right: true, Bottom: false})
	atab.SetCenterSeparator("|")
	headers := []string{}
	for _, c := range tab.columns {
		headers = append(headers, c.(Stringer).String())
	}
	atab.SetHeader(headers)
	for _, r := range tab.rows {
		new := []string{}
		for _, h := range headers {
			new = append(new, fmt.Sprintf("%v", r[h]))
		}
		atab.Append(new)
	}
	return atab
}
