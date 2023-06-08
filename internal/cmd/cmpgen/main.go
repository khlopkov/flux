// cmpgen generates comparison options for the asttest package.
package main

import (
	"fmt"
	"go/importer"
	"go/token"
	"go/types"
	"log"
	"os"
)

func main() {
	if len(os.Args) != 2 {
		log.Println(os.Args)
		fmt.Println("Usage: cmpgen <path to output file>")
		os.Exit(1)
	}
	f, err := os.Create(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	pkg, err := importer.ForCompiler(&token.FileSet{}, "source", nil).Import("github.com/InfluxCommunity/flux/ast")
	if err != nil {
		log.Fatal(err)
	}

	scope := pkg.Scope()

	fmt.Fprintln(f, "package asttest")
	fmt.Fprintln(f)
	fmt.Fprintln(f, "// DO NOT EDIT: This file is autogenerated via the cmpgen command.")
	fmt.Fprintln(f)
	fmt.Fprintln(f, `import (`)
	fmt.Fprintln(f, `	"github.com/google/go-cmp/cmp"`)
	fmt.Fprintln(f, `	"github.com/google/go-cmp/cmp/cmpopts"`)
	fmt.Fprintln(f, `	"github.com/InfluxCommunity/flux/ast"`)
	fmt.Fprintln(f, `)`)
	fmt.Fprintln(f)
	fmt.Fprintln(f, `var IgnoreBaseNodeOptions = []cmp.Option{`)
	for _, name := range scope.Names() {
		obj := scope.Lookup(name)
		if strct, ok := obj.Type().Underlying().(*types.Struct); obj.Exported() && ok {
			for i := 0; i < strct.NumFields(); i++ {
				field := strct.Field(i)
				if field.Name() == "BaseNode" {
					fmt.Fprintf(f, "\tcmpopts.IgnoreFields(ast.%s{}, \"BaseNode\"),\n", obj.Name())
				}
			}
		}
	}
	fmt.Fprintln(f, `}`)
}
