package main

import (
	"fmt"
	"log"
	"reflect"
	"sort"
	"strings"

	"go/ast"
	"go/doc"
	"go/parser"
	"go/token"
)

func parseDoc(apiName, packagePath string) *Section {
	fset := token.NewFileSet()
	pkgs, first := parser.ParseDir(fset, packagePath, nil, parser.ParseComments)
	if first != nil {
		log.Fatalf("parsing code: %s", first)
	}
	for _, pkg := range pkgs {
		docpkg := doc.New(pkg, "", doc.AllDecls)

		for _, t := range docpkg.Types {
			if t.Name == apiName {
				return parseSection(t, docpkg)
			}
		}
	}
	log.Fatalf("type %s not found\n", apiName)
	return nil
}

func cleanText(s string) string {
	return strings.TrimSpace(s)
}

func lookupType(pkg *doc.Package, name string) *doc.Type {
	for _, t := range pkg.Types {
		if t.Name == name {
			return t
		}
	}
	return nil
}

func parseSection(t *doc.Type, pkg *doc.Package) *Section {
	section := &Section{
		pkg,
		t.Name,
		cleanText(t.Doc),
		nil,
		map[string]struct{}{},
		nil,
		nil,
	}
	methods := make([]*doc.Func, len(t.Methods))
	copy(methods, t.Methods)
	sort.Slice(methods, func(i, j int) bool {
		return t.Methods[i].Decl.Name.NamePos < t.Methods[j].Decl.Name.NamePos
	})
	for _, fn := range methods {
		parseMethod(fn, section)
	}

	ts := t.Decl.Specs[0].(*ast.TypeSpec)
	expr := ts.Type
	st := expr.(*ast.StructType)
	for _, f := range st.Fields.List {
		ident, ok := f.Type.(*ast.Ident)
		if !ok {
			continue
		}
		name := ident.Name
		if f.Tag != nil {
			name = reflect.StructTag(stringLiteral(f.Tag.Value)).Get("sherpa")
		}
		subt := lookupType(pkg, ident.Name)
		if subt == nil {
			log.Fatalf("section %s not found", ident.Name)
		}
		subsection := parseSection(subt, pkg)
		subsection.Name = name
		section.Sections = append(section.Sections, subsection)
	}
	return section
}

func gatherFieldType(typeName string, f *Field, e ast.Expr, section *Section) string {
	switch t := e.(type) {
	case *ast.Ident:
		tt := lookupType(section.Pkg, t.Name)
		if tt != nil {
			ensureNamedType(tt, section)
		}
		return t.Name
	case *ast.ArrayType:
		return "[]" + gatherFieldType(typeName, f, t.Elt, section)
	case *ast.MapType:
		return fmt.Sprintf("map[%s]%s", gatherFieldType(typeName, f, t.Key, section), gatherFieldType(typeName, f, t.Value, section))
	case *ast.StructType:
		for _, ft := range t.Fields.List {
			name := nameList(ft.Names, ft.Tag)
			if name == "" {
				continue
			}
			subf := &Field{
				name,
				"",
				fieldDoc(ft),
				[]*Field{},
			}
			subf.Type = gatherFieldType(typeName, subf, ft.Type, section)
			f.Fields = append(f.Fields, subf)
		}
		return "object"
	case *ast.InterfaceType:
		if t.Methods != nil && len(t.Methods.List) > 0 {
			log.Fatalf("unsupported non-empty interface param/return type %T\n", t)
		}
		return "?"
	case *ast.StarExpr:
		return "*" + gatherFieldType(typeName, f, t.X, section)
	case *ast.SelectorExpr:
		// we don't cross package boundaries for docs, eg time.Time
		return t.Sel.Name
	}
	log.Fatalf("unsupported type in struct %s, field %s: %T", e)
	return ""
}

func fieldDoc(f *ast.Field) string {
	s := ""
	if f.Doc != nil {
		s += strings.Replace(strings.TrimSpace(f.Doc.Text()), "\n", " ", -1)
	}
	if f.Comment != nil {
		if s != "" {
			s += "; "
		}
		s += strings.TrimSpace(f.Comment.Text())
	}
	return s
}

// parse type of param/return type used in one of the functions
func ensureNamedType(t *doc.Type, section *Section) {
	if _, have := section.Typeset[t.Name]; have {
		return
	}

	tt := &Type{
		t.Name,
		cleanText(t.Doc),
		[]*Field{},
	}
	// add it early, so self-referencing types can't cause a loop
	section.Types = append(section.Types, tt)
	section.Typeset[tt.Name] = struct{}{}

	ts := t.Decl.Specs[0].(*ast.TypeSpec)
	st, ok := ts.Type.(*ast.StructType)
	if !ok {
		log.Fatalf("unsupported param/return type %T", ts.Type)
	}
	for _, field := range st.Fields.List {
		name := nameList(field.Names, field.Tag)
		if name == "" {
			continue
		}

		f := &Field{
			name,
			"",
			fieldDoc(field),
			[]*Field{},
		}
		f.Type = gatherFieldType(t.Name, f, field.Type, section)
		tt.Fields = append(tt.Fields, f)
	}
}

// todo: there's probably a function in the standard library for this... find it
// parse string literal
func stringLiteral(s string) string {
	if strings.HasPrefix(s, "`") && strings.HasSuffix(s, "`") {
		return s[1 : len(s)-1]
	}
	if strings.HasPrefix(s, `"`) && strings.HasSuffix(s, `"`) {
		return s[1 : len(s)-1]
	}
	return s
}

func jsonName(tag string, name string) string {
	s := reflect.StructTag(tag).Get("json")
	if s == "" {
		return name
	} else if s == "-" {
		return s
	} else {
		return strings.Split(s, ",")[0]
	}
}

func nameList(names []*ast.Ident, tag *ast.BasicLit) string {
	if names == nil {
		return ""
	}
	l := []string{}
	for _, name := range names {
		if ast.IsExported(name.Name) {
			l = append(l, name.Name)
		}
	}
	if len(l) == 1 && tag != nil {
		return jsonName(stringLiteral(tag.Value), l[0])
	}
	return strings.Join(l, ", ")
}

func parseArgType(e ast.Expr, section *Section) string {
	switch t := e.(type) {
	case *ast.Ident:
		tt := lookupType(section.Pkg, t.Name)
		if tt != nil {
			ensureNamedType(tt, section)
		}
		return t.Name
	case *ast.ArrayType:
		return "[]" + parseArgType(t.Elt, section)
	case *ast.Ellipsis:
		return "..." + parseArgType(t.Elt, section)
	case *ast.MapType:
		return fmt.Sprintf("map[%s]%s", parseArgType(t.Key, section), parseArgType(t.Value, section))
	case *ast.StructType:
		l := []string{}
		for _, ft := range t.Fields.List {
			name := nameList(ft.Names, ft.Tag)
			if name == "" {
				continue
			}
			l = append(l, fmt.Sprintf("%s %s", name, parseArgType(ft.Type, section)))
		}
		return fmt.Sprintf("struct{%s}", strings.Join(l, ", "))
	case *ast.InterfaceType:
		if t.Methods != nil && len(t.Methods.List) > 0 {
			log.Fatalf("unsupported non-empty interface param/return type %T\n", t)
		}
		return "?"
	case *ast.StarExpr:
		return "*" + parseArgType(t.X, section)
	case *ast.SelectorExpr:
		// we don't cross package boundaries for docs, eg time.Time
		return t.Sel.Name
	}
	log.Fatalf("unsupported param/return type %T\n", e)
	return ""
}

func parseArgs(isParams bool, fields *ast.FieldList, section *Section) string {
	if fields == nil {
		return ""
	}
	args := []string{}
	for _, f := range fields.List {
		names := []string{}
		for _, name := range f.Names {
			names = append(names, name.Name)
		}
		typeStr := parseArgType(f.Type, section)

		var arg string
		if isParams {
			arg = fmt.Sprintf("%s %s", strings.Join(names, ", "), typeStr)
		} else {
			arg = typeStr
		}
		args = append(args, arg)

	}
	if !isParams && len(args) > 0 && args[len(args)-1] == "error" {
		args = args[:len(args)-1]
	}
	return strings.Join(args, ", ")
}

func lowerFirst(s string) string {
	return strings.ToLower(s[:1]) + s[1:]
}

func parseMethod(fn *doc.Func, section *Section) {
	name := lowerFirst(fn.Name)
	params := parseArgs(true, fn.Decl.Type.Params, section)
	results := parseArgs(false, fn.Decl.Type.Results, section)
	synopsis := fmt.Sprintf("%s(%s)", name, params)
	if results != "" {
		synopsis += " " + results
	}
	f := &Function{
		name,
		synopsis,
		fn.Doc,
	}
	section.Functions = append(section.Functions, f)
}
