package rag

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

// A bounded tree lets shape and table metadata be inspected before their text
// without accepting unbounded XML nesting or following external resources.
type presentationNode struct {
	name     xml.Name
	attrs    []xml.Attr
	children []*presentationNode
	text     strings.Builder
}

func presentationTree(data []byte) (*presentationNode, error) {
	d := xml.NewDecoder(bytes.NewReader(data))
	root := &presentationNode{}
	stack := []*presentationNode{root}
	count := 0
	for {
		token, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := token.(type) {
		case xml.StartElement:
			count++
			if count > 200000 || len(stack) > 128 || len(t.Attr) > 128 {
				return nil, &ParseError{Code: "document_limit", Message: "Presentation XML exceeds supported structural bounds."}
			}
			n := &presentationNode{name: t.Name, attrs: t.Attr}
			parent := stack[len(stack)-1]
			parent.children = append(parent.children, n)
			stack = append(stack, n)
		case xml.EndElement:
			if len(stack) < 2 {
				return nil, fmt.Errorf("invalid presentation XML")
			}
			stack = stack[:len(stack)-1]
		case xml.CharData:
			n := stack[len(stack)-1]
			if n.name.Local == "t" {
				n.text.Write(t)
			}
		}
	}
	if len(root.children) != 1 || len(stack) != 1 {
		return nil, fmt.Errorf("invalid presentation root")
	}
	return root.children[0], nil
}

func (n *presentationNode) attr(name string) string {
	for _, a := range n.attrs {
		if a.Name.Local == name && a.Name.Space == "" {
			return a.Value
		}
	}
	return ""
}
func (n *presentationNode) relationship(name string) string {
	for _, a := range n.attrs {
		if a.Name.Local == name && strings.HasSuffix(a.Name.Space, "/relationships") {
			return a.Value
		}
	}
	return ""
}
func (n *presentationNode) child(name string) *presentationNode {
	for _, c := range n.children {
		if c.name.Local == name {
			return c
		}
	}
	return nil
}
func (n *presentationNode) descendant(name string) *presentationNode {
	if n.name.Local == name {
		return n
	}
	for _, c := range n.children {
		if found := c.descendant(name); found != nil {
			return found
		}
	}
	return nil
}
func (n *presentationNode) drawing() bool {
	return n.name.Space == "" || n.name.Space == "http://schemas.openxmlformats.org/drawingml/2006/main" || n.name.Space == "http://purl.oclc.org/ooxml/drawingml/main"
}
func (n *presentationNode) presentation() bool {
	return n.name.Space == "" || n.name.Space == "http://schemas.openxmlformats.org/presentationml/2006/main" || n.name.Space == "http://purl.oclc.org/ooxml/presentationml/main"
}

func presentationText(n *presentationNode) string {
	var out strings.Builder
	var walk func(*presentationNode)
	walk = func(n *presentationNode) {
		if n.drawing() && n.name.Local == "t" {
			out.WriteString(n.text.String())
			return
		}
		if n.drawing() && n.name.Local == "br" {
			out.WriteByte('\n')
			return
		}
		if n.drawing() && n.name.Local == "tab" {
			out.WriteByte('\t')
			return
		}
		for _, c := range n.children {
			walk(c)
		}
		if n.drawing() && n.name.Local == "p" {
			out.WriteByte('\n')
		}
	}
	walk(n)
	return strings.TrimSpace(out.String())
}
