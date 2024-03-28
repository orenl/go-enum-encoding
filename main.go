package main

import (
	_ "embed"
	"errors"
	"flag"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

//go:embed enum.go.template
var templateCode string

//go:embed enum_test.go.template
var templateTest string

func main() {
	var (
		typeName    string
		fileName    = os.Getenv("GOFILE")
		packageName = os.Getenv("GOPACKAGE")
	)
	flag.StringVar(&typeName, "type", "", "type to be generated for")
	flag.Parse()

	if err := process(typeName, fileName, packageName); err != nil {
		os.Stderr.WriteString(err.Error())
		os.Exit(1)
	}
}

func process(typeName string, fileName string, packageName string) error {
	if typeName == "" || fileName == "" || packageName == "" {
		return errors.New("type, file and package name must be provided")
	}

	inputCode, err := os.ReadFile(fileName)
	if err != nil {
		return err
	}

	f, err := parser.ParseFile(token.NewFileSet(), fileName, inputCode, parser.ParseComments)
	if err != nil {
		return err
	}

	var specs [][2]string

	ast.Inspect(f, func(node ast.Node) bool {
		spec, ok := node.(*ast.ValueSpec)
		if !ok {
			return true
		}

		if len(spec.Names) != 1 {
			return false
		}
		// TODO: check that type matches

		tag, ok := "", false
		for _, field := range strings.Fields(spec.Comment.Text()) {
			if strings.HasPrefix(field, "json:") {
				tag, ok = field[len("json:\""):len(field)-1], true
				break
			}
		}
		if ok {
			specs = append(specs, [2]string{spec.Names[0].Name, tag})
		}
		return false
	})

	sort.Slice(specs, func(i, j int) bool { return specs[i][0] < specs[j][0] })

	code := templateCode
	code = strings.ReplaceAll(code, "{{.Type}}", typeName)
	code = strings.ReplaceAll(code, "{{.Package}}", packageName)
	code = strings.ReplaceAll(code, "{{.val_to_json}}", strings.Join(mp(specs, func(v [2]string) string { return v[0] + `: "` + v[1] + `",` }), "\n"))
	code = strings.ReplaceAll(code, "{{.json_to_value}}", strings.Join(mp(specs, func(v [2]string) string { return `"` + v[1] + `": ` + v[0] + `,` }), "\n"))

	test := templateTest
	test = strings.ReplaceAll(test, "{{.Type}}", typeName)
	test = strings.ReplaceAll(test, "{{.Package}}", packageName)
	test = strings.ReplaceAll(test, "{{.Values}}", strings.Join(mp(specs, func(v [2]string) string { return v[0] }), ", "))
	test = strings.ReplaceAll(test, "{{.Tags}}", strings.Join(mp(specs, func(v [2]string) string { return `"` + v[1] + `"` }), ","))

	return errors.Join(
		writeCode([]byte(code), filepath.Join(filepath.Dir(fileName), strings.ToLower(typeName)+"_enum_encoding.go")),
		writeCode([]byte(test), filepath.Join(filepath.Dir(fileName), strings.ToLower(typeName)+"_enum_encoding_test.go")),
	)
}

func mp[T any, M any](a []T, f func(T) M) (l []M) {
	for _, e := range a {
		l = append(l, f(e))
	}
	return l
}

func writeCode(code []byte, outFilePath string) error {
	formattedCode, err := format.Source(code)
	if err != nil {
		return err
	}
	return os.WriteFile(outFilePath, formattedCode, 0644)
}
