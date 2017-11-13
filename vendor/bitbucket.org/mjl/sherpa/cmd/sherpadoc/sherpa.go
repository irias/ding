package main

import (
	"bitbucket.org/mjl/sherpa"
	"fmt"
	"strings"
)

func generateField(f *Field, indent string) string {
	s := fmt.Sprintf("%s- `%s` _%s_", indent, f.Name, f.Type)
	if f.Doc != "" {
		s += " - " + f.Doc
	}
	s += "\n"
	indent += "\t"
	for _, subf := range f.Fields {
		s += generateField(subf, indent)
	}
	return s
}

func sherpaDoc(section *Section) *sherpa.Doc {
	doc := &sherpa.Doc{
		Title:     section.Name,
		Text:      section.Doc,
		Functions: []*sherpa.FunctionDoc{},
		Sections:  []*sherpa.Doc{},
	}
	for _, t := range section.Types {
		doc.Text += "\n## Type " + t.Name + "\n" + t.Doc + "\n"
		for _, f := range t.Fields {
			doc.Text += generateField(f, "")
		}
	}
	for _, fn := range section.Functions {
		f := &sherpa.FunctionDoc{
			Name: fn.Name,
			Text: strings.TrimSpace(fn.Synopsis + "\n\n" + fn.Doc),
		}
		doc.Functions = append(doc.Functions, f)
	}
	for _, subsec := range section.Sections {
		doc.Sections = append(doc.Sections, sherpaDoc(subsec))
	}
	doc.Text = strings.TrimSpace(doc.Text)
	return doc
}
