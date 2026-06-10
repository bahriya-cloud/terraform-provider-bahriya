package main

import (
	"bytes"
	"embed"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"text/template"
)

//go:embed templates/*.tmpl
var templateFS embed.FS

// render writes a generated Go file for the given resource into outputDir.
// The output is gofmt'd; if formatting fails (because the generated code has
// a syntax error) we write the unformatted source so the error is visible.
func render(res *Resource, outputDir string) error {
	tmpl, err := template.ParseFS(templateFS, "templates/resource.go.tmpl")
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, res); err != nil {
		return fmt.Errorf("execute template for %s: %w", res.Name, err)
	}

	formatted, fmtErr := format.Source(buf.Bytes())
	if fmtErr != nil {
		// Write the raw output so the user can see what went wrong.
		raw := filepath.Join(outputDir, res.Name+"_resource.go.raw")
		_ = os.WriteFile(raw, buf.Bytes(), 0o644)
		return fmt.Errorf("gofmt failed for %s (raw written to %s): %w", res.Name, raw, fmtErr)
	}

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", outputDir, err)
	}
	dst := filepath.Join(outputDir, res.Name+"_resource.go")
	if err := os.WriteFile(dst, formatted, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", dst, err)
	}
	return nil
}
