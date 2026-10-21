package cli

import (
	"fmt"
	"io"
)

// PresentationStyle contains the already-selected terminal tokens for one
// invocation. The OS adapter decides whether color and Unicode are supported.
type PresentationStyle struct {
	Primary   string
	Secondary string
	Success   string
	Error     string
	Accent    string
	Muted     string
	Bold      string
	Dim       string
	Reset     string
	Check     string
	Cross     string
	Bolt      string
	Chevron   string
	Search    string
	Star      string
	Circle    string
	Separator string
	BoxTL     string
	BoxTR     string
	BoxBL     string
	BoxBR     string
	BoxH      string
	BoxV      string
	BoxVR     string
	BoxVL     string
}

type Presenter struct {
	Stdout io.Writer
	Stderr io.Writer
	JSON   bool
	Style  PresentationStyle
}

func (p Presenter) Banner(version string) {
	if p.JSON {
		return
	}
	_, _ = fmt.Fprintln(p.Stdout)
	_, _ = fmt.Fprintf(p.Stdout, "  %sOffGrid LLM%s %s%s%s\n", p.Style.Primary+p.Style.Bold, p.Style.Reset, p.Style.Muted, version, p.Style.Reset)
	_, _ = fmt.Fprintf(p.Stdout, "  %sLocal AI that works offline%s\n", p.Style.Muted, p.Style.Reset)
	_, _ = fmt.Fprintln(p.Stdout)
}

func (p Presenter) Section(title string) {
	if p.JSON {
		return
	}
	_, _ = fmt.Fprintf(p.Stdout, "\n%s%s%s\n\n", p.Style.Primary+p.Style.Bold, title, p.Style.Reset)
}

func (p Presenter) Success(message string) {
	_, _ = fmt.Fprintf(p.Stdout, "%s%s%s %s\n", p.Style.Success, p.Style.Check, p.Style.Reset, message)
}

func (p Presenter) Error(message string) {
	_, _ = fmt.Fprintf(p.Stderr, "%s%s%s %s\n", p.Style.Error, p.Style.Cross, p.Style.Reset, message)
}

func (p Presenter) Info(message string) {
	_, _ = fmt.Fprintf(p.Stdout, "%si%s %s\n", p.Style.Primary, p.Style.Reset, message)
}

func (p Presenter) Warning(message string) {
	_, _ = fmt.Fprintf(p.Stderr, "%s!%s %s\n", p.Style.Accent, p.Style.Reset, message)
}

func (p Presenter) KeyValue(key, value string) {
	if p.JSON {
		return
	}
	_, _ = fmt.Fprintf(p.Stdout, "  %s%-16s%s %s%s%s\n", p.Style.Muted, key, p.Style.Reset, p.Style.Bold, value, p.Style.Reset)
}
