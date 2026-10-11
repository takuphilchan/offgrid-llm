package computer

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const NativeDiscoveryMaxBytes = 4096

// The durable runner adds offgrid_step_id to the model-facing JSON. Reserve
// space for that field (including a maximum-width step ID) inside the limit.
const nativeDiscoveryPayloadBytes = NativeDiscoveryMaxBytes - 64

type ControlQuery struct {
	Observation string `json:"observation_id"`
	Name        string `json:"name,omitempty"`
	Role        string `json:"role,omitempty"`
	Writable    *bool  `json:"writable,omitempty"`
	Invokable   *bool  `json:"invokable,omitempty"`
	Checkable   *bool  `json:"checkable,omitempty"`
	Cursor      string `json:"cursor,omitempty"`
}
type ControlInspection struct {
	Observation string `json:"observation_id"`
	Element     string `json:"element"`
	Cursor      string `json:"cursor,omitempty"`
}
type ControlRow struct {
	NativeElement
	NameLimited bool `json:"name_limited,omitempty"`
	RoleLimited bool `json:"role_limited,omitempty"`
}
type ControlPage struct {
	Version       int          `json:"version"`
	Observation   string       `json:"observation_id"`
	Target        string       `json:"target_id"`
	CapturedAt    time.Time    `json:"captured_at"`
	AgeMS         int64        `json:"age_ms"`
	InputFresh    bool         `json:"input_fresh"`
	SourceLimited bool         `json:"source_limited"`
	ModelOmitted  int          `json:"model_omitted"`
	Total         int          `json:"total_controls"`
	Matches       int          `json:"matches"`
	Controls      []ControlRow `json:"controls"`
	TextOffset    int          `json:"text_offset,omitempty"`
	Next          string       `json:"next_cursor,omitempty"`
}

func validDiscoveryString(s string) bool {
	return len(s) <= 256 && utf8.ValidString(s) && !strings.ContainsRune(s, 0)
}
func (q ControlQuery) Validate() error {
	if !opaqueID(q.Observation) || !validDiscoveryString(q.Name) || !validDiscoveryString(q.Role) || len(q.Cursor) > 256 {
		return ErrInvalidControl
	}
	return nil
}
func (q ControlInspection) Validate() error {
	if !opaqueID(q.Observation) || !opaqueID(q.Element) || len(q.Cursor) > 256 {
		return ErrInvalidControl
	}
	return nil
}
func evidencePage(view NativeView, now time.Time) ControlPage {
	age := now.Sub(view.Observation.CapturedAt).Milliseconds()
	if age < 0 {
		age = 0
	}
	return ControlPage{Version: 1, Observation: view.Observation.ID, Target: view.Observation.Target.ID, CapturedAt: view.Observation.CapturedAt, AgeMS: age, InputFresh: view.Observation.Validate(view.Observation.Target, now) == nil, SourceLimited: view.Limited, Total: len(view.Elements), Controls: []ControlRow{}}
}
func cutControlText(s string, n int) (string, bool) {
	if len(s) <= n {
		return s, false
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n], true
}
func rowFor(e NativeElement) ControlRow {
	r := ControlRow{NativeElement: e}
	r.Name, r.NameLimited = cutControlText(e.Name, 256)
	r.Role, r.RoleLimited = cutControlText(e.Role, 64)
	if e.Text != nil {
		v, cut := cutControlText(*e.Text, 256)
		r.Text = &v
		r.TextLimited = e.TextLimited || cut
	}
	return r
}
func pageBinding(view NativeView, q any) string {
	data, _ := json.Marshal([]any{view, q})
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
func pageCursor(binding string, index int) string {
	return base64.RawURLEncoding.EncodeToString([]byte(binding + ":" + strconv.Itoa(index)))
}
func cursorIndex(cursor, binding string, max int) (int, error) {
	if cursor == "" {
		return 0, nil
	}
	data, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return 0, ErrInvalidControl
	}
	parts := strings.Split(string(data), ":")
	if len(parts) != 2 || parts[0] != binding {
		return 0, ErrStaleObservation
	}
	i, err := strconv.Atoi(parts[1])
	if err != nil || i < 0 || i > max || strconv.Itoa(i) != parts[1] {
		return 0, ErrInvalidControl
	}
	return i, nil
}
func encodedPage(p ControlPage) (string, error) {
	data, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	if len(data) > nativeDiscoveryPayloadBytes {
		return "", fmt.Errorf("control evidence envelope exceeds 4096 bytes; use a smaller observation")
	}
	return string(data), nil
}

// SimpleFold handles Unicode case pairs such as Greek final sigma, not only
// ASCII/lowercasing. No regex, fuzzy identity or executable selector is evaluated.
func foldLiteral(s string) string {
	return strings.Map(func(r rune) rune {
		small := r
		for next := unicode.SimpleFold(r); next != r; next = unicode.SimpleFold(next) {
			if next < small {
				small = next
			}
		}
		return small
	}, s)
}
func FindNativeControls(view NativeView, q ControlQuery, now time.Time) (string, error) {
	if err := q.Validate(); err != nil {
		return "", err
	}
	if q.Observation != view.Observation.ID {
		return "", ErrStaleObservation
	}
	cursor := q.Cursor
	q.Cursor = ""
	binding := pageBinding(view, q)
	indices := []int{}
	name, role := foldLiteral(q.Name), foldLiteral(q.Role)
	for i, e := range view.Elements {
		if !strings.Contains(foldLiteral(e.Name), name) || (role != "" && foldLiteral(e.Role) != role) || q.Writable != nil && e.Writable != *q.Writable || q.Invokable != nil && e.Invokable != *q.Invokable || q.Checkable != nil && e.Checkable != *q.Checkable {
			continue
		}
		indices = append(indices, i)
	}
	start, err := cursorIndex(cursor, binding, len(indices))
	if err != nil {
		return "", err
	}
	p := evidencePage(view, now)
	p.Matches = len(indices)
	end := start
	for end < len(indices) && len(p.Controls) < 16 {
		candidate := p
		candidate.Controls = append(append([]ControlRow(nil), p.Controls...), rowFor(view.Elements[indices[end]]))
		candidate.ModelOmitted = len(indices) - end - 1
		candidate.Next = ""
		if end+1 < len(indices) {
			candidate.Next = pageCursor(binding, end+1)
		}
		if _, err := encodedPage(candidate); err != nil {
			break
		}
		p = candidate
		end++
	}
	if end == start && start < len(indices) {
		return "", fmt.Errorf("control metadata cannot fit within 4096 bytes")
	}
	p.ModelOmitted = len(indices) - end
	if end < len(indices) {
		p.Next = pageCursor(binding, end)
	}
	return encodedPage(p)
}
func InspectNativeControl(view NativeView, q ControlInspection, now time.Time) (string, error) {
	if err := q.Validate(); err != nil {
		return "", err
	}
	if q.Observation != view.Observation.ID {
		return "", ErrStaleObservation
	}
	var e NativeElement
	matches := 0
	for _, item := range view.Elements {
		if item.ID == q.Element {
			e = item
			matches++
		}
	}
	if matches != 1 {
		return "", ErrInvalidControl
	}
	cursor := q.Cursor
	q.Cursor = ""
	binding := pageBinding(view, q)
	text := ""
	if e.Text != nil {
		text = *e.Text
	}
	offset, err := cursorIndex(cursor, binding, len(text))
	if err != nil {
		return "", err
	}
	if offset < len(text) && !utf8.RuneStart(text[offset]) {
		return "", ErrInvalidControl
	}
	p := evidencePage(view, now)
	p.Matches = 1
	p.SourceLimited = p.SourceLimited || e.TextLimited
	p.TextOffset = offset
	r := rowFor(e)
	r.Text = nil
	r.TextLimited = e.TextLimited
	if e.Text == nil {
		p.Controls = []ControlRow{r}
		return encodedPage(p)
	}
	limit := len(text) - offset
	if limit > 2048 {
		limit = 2048
	}
	for {
		part, cut := cutControlText(text[offset:], limit)
		r.Text = &part
		r.TextLimited = e.TextLimited || cut
		p.Controls = []ControlRow{r}
		p.Next = ""
		p.ModelOmitted = len(text) - offset - len(part)
		if p.ModelOmitted > 0 {
			p.Next = pageCursor(binding, offset+len(part))
		}
		result, err := encodedPage(p)
		if err == nil && (len(part) > 0 || offset == len(text)) {
			return result, nil
		}
		if limit <= 4 {
			return "", fmt.Errorf("control metadata cannot fit within 4096 bytes")
		}
		limit /= 2
	}
}
