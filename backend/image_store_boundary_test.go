package backend

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

var imageStoreGuarded = map[string]bool{
	"ImagesBkt":                true,
	"PhotoPersonBkt":           true,
	"VideoTranscriptBkt":       true,
	"ImageByFamilyIndex":       true,
	"ImageByAnchorIndex":       true,
	"ImageByFamilyDateIndex":   true,
	"ImageByPersonDateIndex":   true,
	"ImageApproxByFamilyIndex": true,
	"ImageApproxByPersonIndex": true,
	"PhotoWordIndex":           true,
	"PhotoPersonByPhotoIndex":  true,
	"PhotoPersonByPersonIndex": true,
	"PhotoPersonByFamilyIndex": true,
}

var vboltReads = map[string]bool{
	"Read": true, "ReadSlice": true, "ReadSliceToMap": true, "ReadAppend": true, "ReadToMap": true, "HasKey": true,
	"IterateAll": true, "IterateAllReverse": true, "IterateInBatches": true, "ScanList": true, "IterateBucketFrom": true,
	"ReadTermCount": true, "IterateTerm": true, "ReadTermTargets": true, "ReadTermTargetSingle": true,
	"IterateTarget": true, "IterateAllTerms": true,
}

var imageStoreReaders = map[string]bool{"approxPhotoIds": true, "newPhotoStream": true}

var imageStoreInternals = map[string]bool{
	"reindexImage": true, "writePhotoPersonRow": true, "indexPhotoPersonRow": true, "deletePhotoPersonRow": true,
}

func TestOnlyTheImageStoreWritesImages(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	var bypasses []string
	fset := token.NewFileSet()
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") || file == "image_store.go" {
			continue
		}
		parsed, err := parser.ParseFile(fset, file, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", file, err)
		}
		ast.Inspect(parsed, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			callee := calleeName(call)
			if imageStoreInternals[callee] {
				bypasses = append(bypasses, fmt.Sprintf("%s calls the store's own %s", fset.Position(call.Pos()), callee))
				return true
			}
			if !passesGuarded(call) {
				return true
			}
			if name, ok := strings.CutPrefix(callee, "vbolt."); ok && vboltReads[name] || imageStoreReaders[callee] {
				return true
			}
			bypasses = append(bypasses, fmt.Sprintf("%s writes an image, its people or its transcript through %s", fset.Position(call.Pos()), callee))
			return true
		})
	}
	for _, bypass := range bypasses {
		t.Errorf("%s; use image_store.go", bypass)
	}
}

func passesGuarded(call *ast.CallExpr) bool {
	for _, arg := range call.Args {
		if u, ok := arg.(*ast.UnaryExpr); ok {
			arg = u.X
		}
		if id, ok := arg.(*ast.Ident); ok && imageStoreGuarded[id.Name] {
			return true
		}
	}
	return false
}

func calleeName(call *ast.CallExpr) string {
	fun := call.Fun
	if idx, ok := fun.(*ast.IndexExpr); ok {
		fun = idx.X
	}
	switch f := fun.(type) {
	case *ast.Ident:
		return f.Name
	case *ast.SelectorExpr:
		if pkg, ok := f.X.(*ast.Ident); ok {
			return pkg.Name + "." + f.Sel.Name
		}
		return f.Sel.Name
	}
	return ""
}
