package siat_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// These boundaries apply to both SIAT versions and protect the documented SDK layers.
func TestSDKDependencyBoundaries(t *testing.T) {
	const module = "github.com/supay8/go-siat"
	for _, root := range []string{"internal", "pkg"} {
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
			if err != nil {
				return err
			}
			for _, imp := range file.Imports {
				name, err := strconv.Unquote(imp.Path.Value)
				if err != nil {
					return err
				}
				if strings.Contains(name, "github.com/ron86i/go-siat") {
					t.Errorf("%s imports the old module: %s", path, name)
				}
				forbidden := name == module
				switch {
				case strings.HasPrefix(path, "internal/core/domain/"):
					forbidden = forbidden || strings.HasPrefix(name, module+"/internal/adapter/") || strings.HasPrefix(name, module+"/internal/core/ports") || strings.HasPrefix(name, module+"/pkg/models")
				case strings.HasPrefix(path, "internal/core/ports/"):
					forbidden = forbidden || strings.HasPrefix(name, module+"/internal/adapter/")
				case strings.HasPrefix(path, "pkg/models/"):
					forbidden = forbidden || strings.HasPrefix(name, module+"/internal/adapter/") || strings.HasPrefix(name, module+"/internal/core/ports")
				}
				if forbidden {
					t.Errorf("layer violation: %s imports %s", path, name)
				}
			}
			return nil
		})
		require.NoError(t, err)
	}
}

func TestGeneratedV3SourcesAreReproducible(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Fatal("python3 is required to verify generated sources")
	}
	dir := t.TempDir()
	files := []string{"pkg/models/invoices/v3/generate.py", "pkg/models/v3/generate.py"}
	schemas, err := filepath.Glob("pkg/models/invoices/v3/testdata/xsd/*.xsd")
	require.NoError(t, err)
	files = append(files, schemas...)
	for _, file := range files {
		data, err := os.ReadFile(file)
		require.NoError(t, err)
		target := filepath.Join(dir, file)
		require.NoError(t, os.MkdirAll(filepath.Dir(target), 0700))
		require.NoError(t, os.WriteFile(target, data, 0600))
	}
	for _, path := range []string{"internal/core/ports", "internal/adapter/services"} {
		require.NoError(t, os.MkdirAll(filepath.Join(dir, path), 0700))
	}
	for _, script := range files[:2] {
		cmd := exec.Command("python3", script)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, string(out))
	}
	outputs := []string{
		"internal/core/domain/documents/v3/models_generated.go", "internal/core/domain/documents/v3/catalog_generated.go", "pkg/models/invoices/v3/models_generated.go",
		"internal/core/domain/siat/facturacion/v3/requests_generated.go", "internal/core/domain/siat/facturacion/v3/responses_generated.go", "pkg/models/v3/requests_generated.go", "pkg/models/v3/communication.go", "pkg/models/v3/responses_generated.go", "internal/core/ports/siat_v3_port.go", "internal/adapter/services/siat_v3_methods_generated.go",
	}
	for _, file := range outputs {
		expected, err := os.ReadFile(file)
		require.NoError(t, err)
		actual, err := os.ReadFile(filepath.Join(dir, file))
		require.NoError(t, err)
		require.Equal(t, string(expected), string(actual), "regenerate %s", file)
	}
}
