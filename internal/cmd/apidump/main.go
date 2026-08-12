// Command apidump prints a stable structural snapshot of one Go package API.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"go/importer"
	"go/token"
	"go/types"
	"io"
	"os"
	"os/exec"
	"sort"
)

type listedPackage struct {
	ImportPath string
	Export     string
}

func main() {
	packagePath := flag.String("package", "github.com/liubaicai/esper", "package import path")
	flag.Parse()
	if err := dump(*packagePath, os.Stdout); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func dump(packagePath string, output io.Writer) error {
	command := exec.Command("go", "list", "-deps", "-export", "-json", packagePath)
	stdout, err := command.StdoutPipe()
	if err != nil {
		return err
	}
	command.Stderr = os.Stderr
	if err := command.Start(); err != nil {
		return err
	}
	exports := make(map[string]string)
	decoder := json.NewDecoder(stdout)
	for {
		var item listedPackage
		if err := decoder.Decode(&item); err == io.EOF {
			break
		} else if err != nil {
			return err
		}
		if item.Export != "" {
			exports[item.ImportPath] = item.Export
		}
	}
	if err := command.Wait(); err != nil {
		return err
	}
	lookup := func(path string) (io.ReadCloser, error) {
		file, ok := exports[path]
		if !ok {
			return nil, fmt.Errorf("no export data for %s", path)
		}
		return os.Open(file)
	}
	loaded, err := importer.ForCompiler(token.NewFileSet(), "gc", lookup).Import(packagePath)
	if err != nil {
		return err
	}
	qualifier := func(pkg *types.Package) string {
		if pkg == nil || pkg.Path() == packagePath || pkg.Path() == "github.com/liubaicai/esper/internal/esper" {
			return "esper"
		}
		return pkg.Path()
	}

	lines := make([]string, 0)
	scope := loaded.Scope()
	for _, name := range scope.Names() {
		if !token.IsExported(name) {
			continue
		}
		object := scope.Lookup(name)
		lines = append(lines, objectLine(object, qualifier))
		named, ok := types.Unalias(object.Type()).(*types.Named)
		if !ok {
			continue
		}
		methodSet := types.NewMethodSet(types.NewPointer(named))
		for index := 0; index < methodSet.Len(); index++ {
			selection := methodSet.At(index)
			if selection.Obj().Exported() {
				lines = append(lines, "method "+name+"."+selection.Obj().Name()+" "+types.TypeString(selection.Obj().Type(), qualifier))
			}
		}
	}
	sort.Strings(lines)
	for _, line := range lines {
		if _, err := fmt.Fprintln(output, line); err != nil {
			return err
		}
	}
	return nil
}

func objectLine(object types.Object, qualifier types.Qualifier) string {
	switch object.(type) {
	case *types.Const:
		return "const " + object.Name() + " " + types.TypeString(object.Type(), qualifier)
	case *types.Func:
		return "func " + object.Name() + " " + types.TypeString(object.Type(), qualifier)
	case *types.TypeName:
		return "type " + object.Name() + " " + types.TypeString(object.Type(), qualifier)
	case *types.Var:
		return "var " + object.Name() + " " + types.TypeString(object.Type(), qualifier)
	default:
		return fmt.Sprintf("%T %s %s", object, object.Name(), types.TypeString(object.Type(), qualifier))
	}
}
