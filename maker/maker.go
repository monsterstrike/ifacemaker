package maker

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"strings"

	"golang.org/x/tools/imports"
)

type Method struct {
	Code string
	Docs []string
}

type StructData struct {
	Embedded []string
	Methods  []Method
	Imports  []string
}

func (m *Method) Lines() []string {
	var lines []string
	lines = append(lines, m.Docs...)
	lines = append(lines, m.Code)
	return lines
}

func GetReceiverTypeName(src []byte, fl interface{}) (string, *ast.FuncDecl) {
	fd, ok := fl.(*ast.FuncDecl)
	if !ok {
		return "", nil
	}
	if fd.Recv.NumFields() != 1 {
		return "", nil
	}
	t := fd.Recv.List[0].Type
	st := string(src[t.Pos()-1 : t.End()-1])
	if len(st) > 0 && st[0] == '*' {
		st = st[1:]
	}
	return st, fd
}
func GetStructName(src []byte, dec interface{}) string {

	gd, isGd := dec.(*ast.GenDecl)
	if !isGd || gd.Tok != token.TYPE {
		return ""
	}
	ts, isTs := gd.Specs[0].(*ast.TypeSpec)
	if !isTs {
		return ""
	}
	return ts.Name.Name

}

func GetEmbedded(src []byte, dec interface{}) (embeds []string) {

	gd, isGd := dec.(*ast.GenDecl)
	if !isGd || gd.Tok != token.TYPE {
		return embeds
	}
	ts, isTs := gd.Specs[0].(*ast.TypeSpec)
	if !isTs {
		return embeds
	}
	st, isSt := ts.Type.(*ast.StructType)
	if !isSt {
		return embeds
	}

	for _, l := range st.Fields.List {
		if l.Names == nil {
			embeds = append(embeds, l.Type.(*ast.Ident).Name)
		}
	}

	return embeds
}

func GetParameters(src []byte, fl *ast.FieldList) ([]string, bool) {
	if fl == nil {
		return nil, false
	}
	merged := false
	parts := []string{}

	for _, l := range fl.List {
		names := make([]string, len(l.Names))
		if len(names) > 1 {
			merged = true
		}
		for i, n := range l.Names {
			names[i] = n.Name
		}

		t := string(src[l.Type.Pos()-1 : l.Type.End()-1])

		var v string
		if len(names) > 0 {
			v = fmt.Sprintf("%s %s", strings.Join(names, ", "), t)
			merged = true
		} else {
			v = t
		}
		parts = append(parts, v)

		//log.Println(reflect.TypeOf(l.Type).String())
	}
	return parts, merged || len(parts) > 1
}

func FormatCode(code string) ([]byte, error) {
	opts := &imports.Options{
		TabIndent: true,
		TabWidth:  2,
		Fragment:  true,
		Comments:  true,
	}
	return imports.Process("", []byte(code), opts)
}

func MakeInterface(pkgName, ifaceName string, methods []string, imports []string) ([]byte, error) {
	output := []string{
		"package " + pkgName,
		"import (",
	}
	output = append(output, imports...)
	output = append(output,
		")",
		fmt.Sprintf("type %s interface {", ifaceName),
	)
	output = append(output, methods...)
	output = append(output, "}")
	code := strings.Join(output, "\n")
	return FormatCode(code)
}

type StringSlice []string

func (strs StringSlice) Contain(s string) bool {

	for _, src := range strs {
		if src == s {
			return true
		}
	}

	return false
}

func ParseStruct(src []byte, copyDocs bool, oexs []string) (sdata map[string]*StructData) {

	sdata = make(map[string]*StructData)
	var imports []string

	fset := token.NewFileSet()
	exs := StringSlice(oexs)
	a, err := parser.ParseFile(fset, "", src, parser.ParseComments)
	if err != nil {
		log.Fatal(err.Error())
	}

	for _, i := range a.Imports {
		if i.Name != nil {
			imports = append(imports, fmt.Sprintf("%s %s", i.Name.String(), i.Path.Value))
		} else {
			imports = append(imports, fmt.Sprintf("%s", i.Path.Value))
		}
	}

	for _, d := range a.Decls {

		if name := GetStructName(src, d); name != "" {
			if sdata[name] == nil {
				sdata[name] = &StructData{}
			}
			sdata[name].Embedded = append(sdata[name].Embedded, GetEmbedded(src, d)...)
			continue
		}

		a, fd := GetReceiverTypeName(src, d)
		var methodName string
		if fd != nil {

			methodName = fd.Name.String()
			if methodName[0] > 'Z' {
				continue
			}
			if exs.Contain(methodName) {
				continue
			}

			if sdata[a] == nil {
				sdata[a] = &StructData{}
			}
		} else {

			continue
		}

		params, _ := GetParameters(src, fd.Type.Params)
		ret, merged := GetParameters(src, fd.Type.Results)

		var retValues string
		if merged {
			retValues = fmt.Sprintf("(%s)", strings.Join(ret, ", "))
		} else {
			retValues = strings.Join(ret, ", ")
		}
		method := fmt.Sprintf("%s(%s) %s", methodName, strings.Join(params, ", "), retValues)
		var docs []string
		if fd.Doc != nil && copyDocs {
			for _, d := range fd.Doc.List {
				docs = append(docs, string(src[d.Pos()-1:d.End()-1]))
			}
		}

		sdata[a].Methods = append(sdata[a].Methods, Method{
			Code: method,
			Docs: docs,
		})
		sdata[a].Imports = imports
	}

	return
}
