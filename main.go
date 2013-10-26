package main

import (
	"bitbucket.org/pkg/inflect"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"regexp"
	"strings"
	"text/template"
	"time"
)

type genSpec struct {
	Package   string
	Singular  string
	Plural    string
	Receiver  string
	Loop      string
	Pointer   string
	Generated string
	Command   string
	FileName  string
}

func (g genSpec) String() string {
	return fmt.Sprintf("%s.%s", g.Package, g.Plural)
}

type options struct {
	All          bool
	AllPointer   string
	ExportedOnly bool
}

var opts = options{}

type ArgHandler struct {
	Handle func(string)
}

var allOption = regexp.MustCompile(`-(\*?)a(ll)?`)
var exportedOption = regexp.MustCompile(`-e(xported)?`)
var structArg = regexp.MustCompile(`(\*?)(\p{L}+)\.(\p{L}+)`)

var optionHandlers = []ArgHandler{
	ArgHandler{
		Handle: func(s string) {
			matches := allOption.FindStringSubmatch(s)
			if matches == nil {
				return
			}
			opts.All = true
			opts.AllPointer = matches[1]
		},
	},
	ArgHandler{
		Handle: func(s string) {
			if exportedOption.MatchString(s) {
				opts.ExportedOnly = true
			}
		},
	},
}

var structHandlers = []ArgHandler{
	ArgHandler{
		Handle: func(s string) {
			matches := structArg.FindStringSubmatch(s)

			if matches == nil {
				return
			}

			ptr := matches[1]
			pkg := matches[2]
			typ := matches[3]

			genSpecs = append(genSpecs, newGenSpec(ptr, pkg, typ))
		},
	},
}

var genSpecs = make([]*genSpec, 0)

func main() {
	has_args := len(os.Args) > 1
	if !has_args {
		fmt.Println(usage)
		return
	}

	args := os.Args[1:]

	first := args[0]
	if first == "-help" || first == "help" || first == "?" {
		fmt.Println(usage)
		return
	}

	getOptions(args)
	getStructs(args)

	if opts.All {
		getAllStructs()
	}

	t := getTemplate()
	writeFile(t, genSpecs)
}

func getOptions(args []string) {
	for _, a := range args {
		for _, h := range optionHandlers {
			h.Handle(a)
		}
	}
}

func getStructs(args []string) {
	for _, a := range args {
		for _, h := range optionHandlers {
			h.Handle(a)
		}
	}
}

func newGenSpec(ptr, pkg, typ string) *genSpec {
	typ = inflect.Singularize(typ)
	return &genSpec{
		Pointer:   ptr,
		Package:   pkg,
		Singular:  typ,
		Plural:    inflect.Pluralize(typ),
		Receiver:  "rcv",
		Loop:      "_item",
		Generated: time.Now().UTC().Format(time.RFC1123),
		Command:   fmt.Sprintf("%s %s%s.%s", "gen", ptr, pkg, typ),
		FileName:  strings.ToLower(typ) + "_gen.go",
	}
}

var goFiles = func(f os.FileInfo) bool {
	return strings.HasSuffix(f.Name(), ".go")
}

func getAllStructs() {
	fset := token.NewFileSet()

	dir, err := parser.ParseDir(fset, "./", goFiles, parser.ParseComments)
	if err != nil {
		fmt.Println(err)
		return
	}

	for pkg, f := range dir {
		ast.Inspect(f, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.TypeSpec:
				switch y := x.Type.(type) {
				case *ast.StructType:
					_ = y
					typ := x.Name.String()
					if !opts.ExportedOnly || ast.IsExported(typ) {
						genSpecs = append(genSpecs, newGenSpec(opts.AllPointer, pkg, typ))
					}
				}
			}
			return true
		})
	}
}

func writeFile(t *template.Template, genSpecs []*genSpec) {
	for _, v := range genSpecs {
		f, err := os.Create(v.FileName)
		if err != nil {
			panic(err)
		}
		defer f.Close()

		t.Execute(f, v)
		fmt.Printf("  generated %s, yay!\n", v)
	}
}

const usage = `Usage: gen [[*]package.TypeName] [-[*]all] [-exported]

  *package.TypeName    # generate funcs for specified struct type; use leading * to specify pointer type (recommended)
  -all                 # generate all structs in current directory; use leading * to specify pointer type (recommended); shortcut -a or -*a
  -exported            # only generate exported structs; shortcut -e
`
