package rag

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"path"
	"strings"
)

type officePackage struct {
	files map[string]*zip.File
	limit int64
}

func (p *DocumentParser) officePackage(content []byte) (*officePackage, error) {
	r, err := p.openArchive(content)
	if err != nil {
		return nil, err
	}
	pkg := &officePackage{files: make(map[string]*zip.File), limit: p.maxExtractedSize}
	for _, f := range r.File {
		if _, exists := pkg.files[f.Name]; exists {
			return nil, fmt.Errorf("duplicate Office part")
		}
		pkg.files[f.Name] = f
	}
	return pkg, nil
}

func (p *officePackage) read(name string) ([]byte, error) {
	f := p.files[name]
	if f == nil {
		return nil, fmt.Errorf("missing Office part")
	}
	r, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return readParserBytes(r, p.limit)
}

func (p *officePackage) decode(name string, v any) error {
	data, err := p.read(name)
	if err != nil {
		return err
	}
	return xml.Unmarshal(data, v)
}

type officeRelation struct {
	ID     string `xml:"Id,attr"`
	Type   string `xml:"Type,attr"`
	Target string `xml:"Target,attr"`
	Mode   string `xml:"TargetMode,attr"`
}

func (p *officePackage) relations(main string) (map[string]officeRelation, error) {
	var doc struct {
		Items []officeRelation `xml:"Relationship"`
	}
	err := p.decode(path.Join(path.Dir(main), "_rels", path.Base(main)+".rels"), &doc)
	if err != nil {
		return nil, err
	}
	result := make(map[string]officeRelation)
	for _, item := range doc.Items {
		if item.ID == "" {
			return nil, fmt.Errorf("empty Office relationship")
		}
		if _, exists := result[item.ID]; exists {
			return nil, fmt.Errorf("duplicate Office relationship")
		}
		result[item.ID] = item
	}
	return result, nil
}

func officePart(main, id, kind string, relations map[string]officeRelation) (string, error) {
	r, ok := relations[id]
	if !ok || r.Mode != "" && r.Mode != "Internal" || !strings.HasSuffix(r.Type, "/"+kind) || strings.ContainsAny(r.Target, "\\:?#") {
		return "", fmt.Errorf("invalid or external Office relationship")
	}
	target := path.Join(path.Dir(main), r.Target)
	if strings.HasPrefix(r.Target, "/") {
		target = path.Clean(strings.TrimPrefix(r.Target, "/"))
	}
	root := strings.Split(main, "/")[0] + "/"
	if !strings.HasPrefix(target, root) {
		return "", fmt.Errorf("Office relationship outside package family")
	}
	return target, nil
}
