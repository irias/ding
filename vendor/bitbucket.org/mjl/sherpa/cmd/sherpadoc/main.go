/*
Sherpadoc reads your Go code, and prints sherpa documentation in JSON.

Example:

	sherpadoc Awesome >awesome.json

Sherpadoc parses the Go code, finds the type (a struct) "Awesome", and gathers documentation:

Comments above the struct are used as section documentation.  Fields in section structs cause the referenced section struct to be included in the generated documentation as well. Set the name of the (sub)section using a struct tag "sherpa", for example `sherpa:"Another Awesome API"`.

Comments above method names are function documentation. A synopsis is automatically generated.

Types used as parameters or return values are added to the section documentation where they are used. The comments above the type are used, as well as the comments for each field in a struct.  The documented field names know about the "json" struct field tags.

	Usage: sherpadoc main-section-api-type
	  -package-path string
		of source code to parse (default ".")
	  -title string
		title of the API, default is the name of the type of the main API
*/
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"go/doc"
)

var (
	packagePath = flag.String("package-path", ".", "of source code to parse")
	title       = flag.String("title", "", "title of the API, default is the name of the type of the main API")
)

type Field struct {
	Name   string
	Type   string
	Doc    string
	Fields []*Field
}

type Type struct {
	Name   string
	Doc    string
	Fields []*Field
}

type Function struct {
	Name     string
	Synopsis string
	Doc      string
}

type Section struct {
	Pkg       *doc.Package
	Name      string
	Doc       string
	Types     []*Type
	Typeset   map[string]struct{}
	Functions []*Function
	Sections  []*Section
}

func main() {
	log.SetPrefix("sherpadoc: ")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: sherpadoc main-section-api-type")
		flag.PrintDefaults()
	}
	flag.Parse()
	args := flag.Args()
	if len(args) != 1 {
		flag.Usage()
		os.Exit(2)
	}
	section := parseDoc(args[0], *packagePath)
	if *title != "" {
		section.Name = *title
	}

	// move types used in multiple sections to the top
	typeCounts := map[string]int{}
	countTypes(typeCounts, section)
	moved := map[string]struct{}{}
	for _, t := range section.Types {
		moved[t.Name] = struct{}{}
	}
	for _, subsec := range section.Sections {
		moveTypes(typeCounts, moved, subsec, section)
	}

	doc := sherpaDoc(section)
	writeJSON(doc)
}

func writeJSON(v interface{}) {
	buf, err := json.MarshalIndent(v, "", "\t")
	if err != nil {
		log.Fatal(err)
	}
	_, err = os.Stdout.Write(buf)
	if err == nil {
		_, err = fmt.Println()
	}
	if err != nil {
		log.Fatal(err)
	}
}

func countTypes(counts map[string]int, section *Section) {
	for _, t := range section.Types {
		counts[t.Name] += 1
	}
	for _, subsec := range section.Sections {
		countTypes(counts, subsec)
	}
}

// todo: only move up to the common section, not always to the top section
func moveTypes(typeCounts map[string]int, moved map[string]struct{}, section, topSection *Section) {
	var ntypes []*Type
	for _, t := range section.Types {
		if typeCounts[t.Name] <= 1 {
			ntypes = append(ntypes, t)
			continue
		}
		_, ok := moved[t.Name]
		if !ok {
			moved[t.Name] = struct{}{}
			topSection.Types = append(topSection.Types, t)
			topSection.Typeset[t.Name] = struct{}{}
		}
	}
	section.Types = ntypes
	for _, subsec := range section.Sections {
		moveTypes(typeCounts, moved, subsec, topSection)
	}
}
