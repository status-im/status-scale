package scale

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
)

type asciiTable struct {
	tab *tabwriter.Writer
}

func (t *asciiTable) AddHeaders(headers ...string) error {
	_, err := fmt.Fprintf(t.tab, "|%s|\n", strings.Join(headers, "\t|"))
	if err != nil {
		return err
	}
	lines := make([]string, len(headers))
	for i := range lines {
		lines[i] = "-"
	}
	_, err = fmt.Fprintf(t.tab, "|%s|\n", strings.Join(lines, "\t|"))
	return err
}

func (t *asciiTable) AddRow(row ...string) error {
	_, err := fmt.Fprintf(t.tab, "|%s|\n", strings.Join(row, "\t|"))
	return err
}

func (t *asciiTable) Flush() error {
	return t.tab.Flush()
}

func newASCIITable(w io.Writer) *asciiTable {
	tab := tabwriter.NewWriter(w, 0, 8, 1, '\t', 0)
	return &asciiTable{tab: tab}
}

// Report represents stats collected from each node.
type Report struct {
	NewEnvelopes float64
	OldEnvelopes float64
	Ingress      float64
	Egress       float64
	IngressAdv   float64
	EgressAdv    float64
	FalseAdv     float64
}

// Summary is a slice of stats collected from each node.
type Summary []Report

// MeanOldPerNew returns mean number of old envelopes per new envelopes ratio.
func (s Summary) MeanOldPerNew() float64 {
	var sum float64
	for _, r := range s {
		sum += r.OldEnvelopes / r.NewEnvelopes
	}
	return sum / float64(len(s))
}

// Print writes a summary to a given writer.
func (s Summary) Print(w io.Writer) error {
	var (
		ingress   float64
		egress    float64
		newEnv    float64
		oldEnv    float64
		oldPerNew = s.MeanOldPerNew()
		iadv      float64
		eadv      float64
		fadv      float64
	)
	tab := newASCIITable(w)
	_, err := fmt.Fprintln(w, "=== SUMMARY")
	if err != nil {
		return err
	}
	if err := tab.AddHeaders("HEADERS", "ingress", "egress", "dups", "new", "dups/new", "iadv", "eadv", "fadv"); err != nil {
		return err
	}
	for i, r := range s {
		ingress += r.Ingress
		egress += r.Egress
		newEnv += r.NewEnvelopes
		oldEnv += r.OldEnvelopes
		iadv += r.IngressAdv
		eadv += r.EgressAdv
		fadv += r.FalseAdv
		if err := tab.AddRow(
			fmt.Sprintf("%d", i),
			fmt.Sprintf("%f mb", r.Ingress/1024/1024),
			fmt.Sprintf("%f mb", r.Egress/1024/1024),
			fmt.Sprintf("%d", int64(r.OldEnvelopes)),
			fmt.Sprintf("%d", int64(r.NewEnvelopes)),
			fmt.Sprintf("%f", r.OldEnvelopes/r.NewEnvelopes),
			fmt.Sprintf("%f mb", r.IngressAdv/1024/1024),
			fmt.Sprintf("%f mb", r.EgressAdv/1024/1024),
			fmt.Sprintf("%d", int64(r.FalseAdv)),
		); err != nil {
			return err
		}
	}
	ingress = ingress / 1024 / 1024
	egress = egress / 1024 / 1024
	if err := tab.AddRow(
		"TOTAL",
		fmt.Sprintf("%f mb", ingress),
		fmt.Sprintf("%f mb", egress),
		fmt.Sprintf("%d", int64(oldEnv)),
		fmt.Sprintf("%d", int64(newEnv)),
		fmt.Sprintf("%f", oldPerNew),
		fmt.Sprintf("%f mb", iadv/1024/1024),
		fmt.Sprintf("%f mb", eadv/1024/1024),
		fmt.Sprintf("%d", int64(fadv)),
	); err != nil {
		return err
	}
	return tab.Flush()
}
