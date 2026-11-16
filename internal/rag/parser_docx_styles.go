package rag

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type wordStyle struct {
	parent    string
	level     *int
	paragraph bool
}

func wordAttribute(t xml.StartElement, key string) string {
	for _, a := range t.Attr {
		if wordName(a.Name) && a.Name.Local == key {
			return a.Value
		}
	}
	return ""
}

// Resolve declared outline levels, not localized style names. Missing or
// different-type parents are ignored as prescribed by WordprocessingML.
// All sources are bounded package parts; external relationships are rejected.
func wordStyleLevels(pkg *officePackage) (map[string]int, error) {
	part := "word/styles.xml"
	if pkg.files["word/_rels/document.xml.rels"] != nil {
		relations, err := pkg.relations("word/document.xml")
		if err != nil {
			return nil, err
		}
		found := false
		for id, r := range relations {
			if strings.HasSuffix(r.Type, "/styles") {
				if found {
					return nil, fmt.Errorf("ambiguous Word styles relationship")
				}
				found = true
				part, err = officePart("word/document.xml", id, "styles", relations)
				if err != nil {
					return nil, err
				}
			}
		}
		if found && pkg.files[part] == nil {
			return nil, fmt.Errorf("missing Word styles part")
		}
	}
	if pkg.files[part] == nil {
		return map[string]int{}, nil
	}
	data, err := pkg.read(part)
	if err != nil {
		return nil, err
	}
	decoder := xml.NewDecoder(bytes.NewReader(data))
	stack := []xml.Name{}
	styles := map[string]wordStyle{}
	current, defaultID := "", ""
	roots := 0
	documentDefault := 0
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := token.(type) {
		case xml.Directive:
			return nil, fmt.Errorf("Word style declarations are not supported")
		case xml.StartElement:
			stack = append(stack, t.Name)
			if len(stack) > 64 {
				return nil, fmt.Errorf("Word style nesting exceeds limit")
			}
			if len(stack) == 1 {
				roots++
				if roots != 1 || !wordName(t.Name) || t.Name.Local != "styles" {
					return nil, fmt.Errorf("invalid Word styles root")
				}
			}
			if !wordName(t.Name) {
				continue
			}
			if len(stack) == 2 && t.Name.Local == "style" {
				current = wordAttribute(t, "styleId")
				if current == "" || len(current) > 256 || len(styles) >= 4096 {
					return nil, fmt.Errorf("invalid or excessive Word styles")
				}
				if _, exists := styles[current]; exists {
					return nil, fmt.Errorf("duplicate Word style")
				}
				paragraph := wordAttribute(t, "type") == "paragraph"
				styles[current] = wordStyle{paragraph: paragraph}
				flag := wordAttribute(t, "default")
				if paragraph && (flag == "1" || flag == "true" || flag == "on") {
					if defaultID != "" {
						return nil, fmt.Errorf("ambiguous default Word style")
					}
					defaultID = current
				}
			}
			if current != "" && len(stack) == 3 && t.Name.Local == "basedOn" {
				s := styles[current]
				if s.parent != "" {
					return nil, fmt.Errorf("duplicate Word parent style")
				}
				s.parent = wordAttribute(t, "val")
				styles[current] = s
			}
			styleOutline := current != "" && len(stack) == 4 && wordName(stack[2]) && stack[2].Local == "pPr"
			defaultOutline := len(stack) == 5 && wordName(stack[1]) && stack[1].Local == "docDefaults" && wordName(stack[2]) && stack[2].Local == "pPrDefault" && wordName(stack[3]) && stack[3].Local == "pPr"
			if t.Name.Local == "outlineLvl" && (styleOutline || defaultOutline) {
				n, err := strconv.Atoi(wordAttribute(t, "val"))
				if err != nil || n < 0 || n > 9 {
					return nil, fmt.Errorf("invalid Word outline level")
				}
				level := n + 1
				if n == 9 {
					level = 0
				}
				if defaultOutline {
					documentDefault = level
				} else {
					s := styles[current]
					if s.level != nil {
						return nil, fmt.Errorf("duplicate Word outline level")
					}
					s.level = &level
					styles[current] = s
				}
			}
		case xml.EndElement:
			if len(stack) == 2 {
				current = ""
			}
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		case xml.CharData:
			if len(stack) == 0 && strings.TrimSpace(string(t)) != "" {
				return nil, fmt.Errorf("text outside Word styles")
			}
		}
	}
	if roots != 1 || len(stack) != 0 {
		return nil, fmt.Errorf("incomplete Word styles")
	}
	levels := map[string]int{}
	visiting := map[string]bool{}
	var resolve func(string, int) (int, error)
	resolve = func(id string, depth int) (int, error) {
		if v, ok := levels[id]; ok {
			return v, nil
		}
		style, exists := styles[id]
		if !exists || !style.paragraph {
			return documentDefault, nil
		}
		if visiting[id] || depth > 64 {
			return 0, fmt.Errorf("cyclic or excessive Word style inheritance")
		}
		visiting[id] = true
		level := documentDefault
		if parent, ok := styles[style.parent]; ok && parent.paragraph {
			var err error
			level, err = resolve(style.parent, depth+1)
			if err != nil {
				return 0, err
			}
		}
		if style.level != nil {
			level = *style.level
		}
		visiting[id] = false
		levels[id] = level
		return level, nil
	}
	for id, style := range styles {
		if style.paragraph {
			if _, err := resolve(id, 0); err != nil {
				return nil, err
			}
		}
	}
	levels[""] = documentDefault
	if defaultID != "" {
		levels[""] = levels[defaultID]
	}
	return levels, nil
}
