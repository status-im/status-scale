package metrics

import (
	"errors"
	"fmt"

	"github.com/buger/jsonparser"
)

type Row map[string]interface{}

type UIDColumn struct {
	Header string
}

func (c UIDColumn) String() string {
	return c.Header
}

type RawColumn struct {
	Path   []string
	Header string
}

func (r RawColumn) String() string {
	return r.Header
}

func (r RawColumn) Compute(data []byte) (int64, error) {
	rst, err := jsonparser.GetInt(data, r.Path...)
	if err != nil {
		return rst, fmt.Errorf("error getting path %v, %v", r.Path, err)
	}
	return rst, nil
}

type ComputeColumn struct {
	Header string
	Handle func(r Row) (interface{}, error)
}

func (s ComputeColumn) String() string {
	return s.Header
}

func (s ComputeColumn) Compute(r Row) (interface{}, error) {
	return s.Handle(r)
}

func NewTab() *Table {
	return new(Table)
}

func NewCompleteTab(uid string, columns ...[]interface{}) *Table {
	tab := NewTab()
	tab.AddColumns(UIDColumn{uid})
	for i := range columns {
		tab.AddColumns(columns[i]...)
	}
	return tab
}

type Table struct {
	columns []interface{}
	rows    []Row
}

func (t *Table) AddColumns(columns ...interface{}) {
	t.columns = append(t.columns, columns...)
}

func (t *Table) Append(uid string, data []byte) error {
	r := Row{}
	for i := range t.columns {
		col := t.columns[i]
		switch v := col.(type) {
		case UIDColumn:
			r[v.String()] = uid
		case RawColumn:
			rst, err := v.Compute(data)
			if err != nil {
				return err
			}
			r[v.String()] = rst
		case ComputeColumn:
			// pass copy?
			rst, err := v.Compute(r)
			if err != nil {
				return err
			}
			r[v.String()] = rst
		default:
			return errors.New("not supported column type")
		}
	}
	t.rows = append(t.rows, r)
	return nil
}
