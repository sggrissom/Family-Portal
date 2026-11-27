package backend

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"
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

var imageStoreBypasses = map[string]bool{
	"account_deletion.go:deletePersonRecordTx":     true,
	"archive_photos.go:importPhotoLinks":           true,
	"archive_photos.go:importPhotos":               true,
	"archive_videos.go:importVideos":               true,
	"calendar_dates.go:NormalizeStoredRecordDates": true,
	"faces.go:ensurePhotoPersonTx":                 true,
	"person.go:MergePeople":                        true,
	"photo_index.go:ReindexPhotoDates":             true,
	"photos.go:deletePhotoRecordTx":                true,
	"photos.go:uploadPhotoHandler":                 true,
	"record_time_store.go:writeImage":              true,
	"seed_photos.go:photo":                         true,
	"video_transcripts.go:deleteVideoTranscriptTx": true,
	"video_uploads.go:CompleteVideoUpload":         true,
}

func TestOnlyTheImageStoreWritesImages(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]bool{}
	fset := token.NewFileSet()
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") || file == "image_store.go" {
			continue
		}
		parsed, err := parser.ParseFile(fset, file, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", file, err)
		}
		for _, decl := range parsed.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok || !passesGuarded(call) {
					return true
				}
				callee := calleeName(call)
				if name, ok := strings.CutPrefix(callee, "vbolt."); ok && vboltReads[name] || imageStoreReaders[callee] {
					return true
				}
				found[file+":"+fn.Name.Name] = true
				return true
			})
		}
	}

	var unexpected, gone []string
	for site := range found {
		if !imageStoreBypasses[site] {
			unexpected = append(unexpected, site)
		}
	}
	for site := range imageStoreBypasses {
		if !found[site] {
			gone = append(gone, site)
		}
	}
	sort.Strings(unexpected)
	sort.Strings(gone)
	for _, site := range unexpected {
		t.Errorf("%s writes an image, its people or its transcript outside image_store.go", site)
	}
	for _, site := range gone {
		t.Errorf("%s no longer bypasses the image store; drop it from imageStoreBypasses", site)
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
