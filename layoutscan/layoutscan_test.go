//go:build !wasm

package layoutscan

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, root, relPath, content string) {
	t.Helper()
	fullPath := filepath.Join(root, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func hasRule(violations []Violation, rule string) bool {
	for _, v := range violations {
		if v.Rule == rule {
			return true
		}
	}
	return false
}

// TestR1_ForbiddenFileName: modules/m/init.go triggers RuleForbiddenFileName
func TestR1_ForbiddenFileName(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "modules/m/init.go", "package m\n")

	v := VerifyLayout(dir)
	if !hasRule(v, RuleForbiddenFileName) {
		t.Errorf("VerifyLayout = %+v; want rule %s", v, RuleForbiddenFileName)
	}
}

// TestR1_Negative: valid files modules/m/{module,server,browser}.go + modules/m/staffpicker.go produce 0 violations
func TestR1_Negative(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "modules/m/module.go", "package m\n")
	writeFile(t, dir, "modules/m/server.go", "//go:build !browser\npackage m\n")
	writeFile(t, dir, "modules/m/browser.go", "//go:build browser\npackage m\n")
	writeFile(t, dir, "modules/m/staffpicker.go", "package m\n")

	v := VerifyLayout(dir)
	if len(v) != 0 {
		t.Errorf("VerifyLayout = %+v; want 0 violations", v)
	}
}

// TestR2_Subdirectory: modules/m/sub/x.go triggers RuleModuleSubdirectory
func TestR2_Subdirectory(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "modules/m/sub/x.go", "package sub\n")

	v := VerifyLayout(dir)
	if !hasRule(v, RuleModuleSubdirectory) {
		t.Errorf("VerifyLayout = %+v; want rule %s", v, RuleModuleSubdirectory)
	}
}

// TestR2_DocsExempt: modules/m/docs/README.md produces 0 violations
func TestR2_DocsExempt(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "modules/m/module.go", "package m\n")
	writeFile(t, dir, "modules/m/server.go", "//go:build !browser\npackage m\n")
	writeFile(t, dir, "modules/m/browser.go", "//go:build browser\npackage m\n")
	writeFile(t, dir, "modules/m/docs/README.md", "# Module Docs\n")

	v := VerifyLayout(dir)
	if len(v) != 0 {
		t.Errorf("VerifyLayout = %+v; want 0 violations", v)
	}
}

// TestR2_DocsNestedExempt: modules/m/docs/reference/x.md produces 0 violations
func TestR2_DocsNestedExempt(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "modules/m/module.go", "package m\n")
	writeFile(t, dir, "modules/m/server.go", "//go:build !browser\npackage m\n")
	writeFile(t, dir, "modules/m/browser.go", "//go:build browser\npackage m\n")
	writeFile(t, dir, "modules/m/docs/reference/x.md", "# Reference\n")

	v := VerifyLayout(dir)
	if len(v) != 0 {
		t.Errorf("VerifyLayout = %+v; want 0 violations", v)
	}
}

// TestR2_DataNotExempt: modules/m/data/seed.sql triggers RuleModuleSubdirectory
func TestR2_DataNotExempt(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "modules/m/module.go", "package m\n")
	writeFile(t, dir, "modules/m/server.go", "//go:build !browser\npackage m\n")
	writeFile(t, dir, "modules/m/browser.go", "//go:build browser\npackage m\n")
	writeFile(t, dir, "modules/m/data/seed.sql", "-- seed\n")

	v := VerifyLayout(dir)
	if len(v) != 1 || v[0].Rule != RuleModuleSubdirectory {
		t.Errorf("VerifyLayout = %+v; want 1 violation of %s", v, RuleModuleSubdirectory)
	}
}

// TestR2_DocsPrefixNotExempt: modules/m/docs_old/x.md triggers RuleModuleSubdirectory
func TestR2_DocsPrefixNotExempt(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "modules/m/module.go", "package m\n")
	writeFile(t, dir, "modules/m/server.go", "//go:build !browser\npackage m\n")
	writeFile(t, dir, "modules/m/browser.go", "//go:build browser\npackage m\n")
	writeFile(t, dir, "modules/m/docs_old/x.md", "old docs\n")

	v := VerifyLayout(dir)
	if len(v) != 1 || v[0].Rule != RuleModuleSubdirectory {
		t.Errorf("VerifyLayout = %+v; want 1 violation of %s", v, RuleModuleSubdirectory)
	}
}

// TestR3_MissingBuildTag: modules/m/browser.go without tag triggers RuleMissingBuildTag
func TestR3_MissingBuildTag(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "modules/m/browser.go", "package m\n")

	v := VerifyLayout(dir)
	if !hasRule(v, RuleMissingBuildTag) {
		t.Errorf("VerifyLayout = %+v; want rule %s", v, RuleMissingBuildTag)
	}
}

// TestR3_FalsePositive: server.go with correct //go:build !browser header and mid-file comment has 0 violations
func TestR3_FalsePositive(t *testing.T) {
	dir := t.TempDir()
	src := `//go:build !browser
package m

// comment mentioning //go:build wasm inside body
func Foo() {}
`
	writeFile(t, dir, "modules/m/server.go", src)

	v := VerifyLayout(dir)
	if len(v) != 0 {
		t.Errorf("VerifyLayout = %+v; want 0 violations", v)
	}
}

// TestR4_WrongBuildTag: modules/m/browser.go with //go:build wasm triggers RuleWrongBuildTag
func TestR4_WrongBuildTag(t *testing.T) {
	dir := t.TempDir()
	src := `//go:build wasm
package m
`
	writeFile(t, dir, "modules/m/browser.go", src)

	v := VerifyLayout(dir)
	if !hasRule(v, RuleWrongBuildTag) {
		t.Errorf("VerifyLayout = %+v; want rule %s", v, RuleWrongBuildTag)
	}
}

// TestR5_TestOutsideTests: modules/m/m_test.go triggers RuleTestOutsideTests
func TestR5_TestOutsideTests(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "modules/m/m_test.go", "package m\n")

	v := VerifyLayout(dir)
	if !hasRule(v, RuleTestOutsideTests) {
		t.Errorf("VerifyLayout = %+v; want rule %s", v, RuleTestOutsideTests)
	}
}

// TestR5_Negative: tests/m_test.go produces 0 violations
func TestR5_Negative(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "tests/m_test.go", "package tests\n")

	v := VerifyLayout(dir)
	if len(v) != 0 {
		t.Errorf("VerifyLayout = %+v; want 0 violations", v)
	}
}

// TestR6_UntypedResult: modules/m/browser.go with func F() []any triggers RuleUntypedResult
func TestR6_UntypedResult(t *testing.T) {
	dir := t.TempDir()
	src := `package m

func F() []any {
	return nil
}
`
	writeFile(t, dir, "modules/m/browser.go", src)

	v := VerifyLayout(dir)
	if !hasRule(v, RuleUntypedResult) {
		t.Errorf("VerifyLayout = %+v; want rule %s", v, RuleUntypedResult)
	}
}

// TestR7_RegisterArity: config/routes.go with Register(r router.Router, extra int) triggers RuleRegisterArity
func TestR7_RegisterArity(t *testing.T) {
	dir := t.TempDir()
	src := `package config

import "webtyp.com/router"

func Register(r router.Router, extra int) {
}
`
	writeFile(t, dir, "config/routes.go", src)

	v := VerifyLayout(dir)
	if !hasRule(v, RuleRegisterArity) {
		t.Errorf("VerifyLayout = %+v; want rule %s", v, RuleRegisterArity)
	}
}

// TestR7_ValidRegister: config/routes.go with Register(r router.Router) produces 0 violations
func TestR7_ValidRegister(t *testing.T) {
	dir := t.TempDir()
	src := `package config

import "webtyp.com/router"

func Register(r router.Router) {
}
`
	writeFile(t, dir, "config/routes.go", src)

	v := VerifyLayout(dir)
	if len(v) != 0 {
		t.Errorf("VerifyLayout = %+v; want 0 violations", v)
	}
}

// TestAccumulation: tree with violations of R1, R2 and R5 produces 3 violations
func TestAccumulation(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "modules/m/init.go", "package m\n")       // R1
	writeFile(t, dir, "modules/m/server.go", "//go:build !browser\npackage m\n")
	writeFile(t, dir, "modules/m/browser.go", "//go:build browser\npackage m\n")
	writeFile(t, dir, "modules/m/sub/x.go", "package sub\n")    // R2
	writeFile(t, dir, "modules/m/m_test.go", "package m_test\n") // R5

	v := VerifyLayout(dir)
	if len(v) != 3 {
		t.Fatalf("VerifyLayout returned %d violations, want 3: %+v", len(v), v)
	}
	if !hasRule(v, RuleForbiddenFileName) || !hasRule(v, RuleModuleSubdirectory) || !hasRule(v, RuleTestOutsideTests) {
		t.Errorf("VerifyLayout = %+v; expected R1, R2, R5", v)
	}
}


func TestT1(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "modules/a/server.go", "//go:build !browser\n\npackage m\n")
	writeFile(t, dir, "modules/a/browser.go", "//go:build browser\n\npackage m\n")
	writeFile(t, dir, "modules/a/module.go", "package m\n")
	writeFile(t, dir, "modules/a/css.go", "//go:build !wasm\n\npackage m\n")
	v := VerifyLayout(dir)
	if len(v) != 0 {
		t.Errorf("VerifyLayout = %+v; want 0 violations", v)
	}
}

func TestT2(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "modules/a/server.go", "//go:build !wasm\n\npackage m\n")
	v := VerifyLayout(dir)
	if len(v) != 1 || v[0].Rule != RuleWrongBuildTag {
		t.Errorf("VerifyLayout = %+v; want 1 %s", v, RuleWrongBuildTag)
	}
}

func TestT3(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "modules/a/browser.go", "package m\n")
	v := VerifyLayout(dir)
	if len(v) != 1 || v[0].Rule != RuleMissingBuildTag {
		t.Errorf("VerifyLayout = %+v; want 1 %s", v, RuleMissingBuildTag)
	}
}

func TestT4(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "modules/a/module.go", "//go:build browser\n\npackage m\n")
	v := VerifyLayout(dir)
	if len(v) != 1 || v[0].Rule != RuleUnexpectedBuildTag {
		t.Errorf("VerifyLayout = %+v; want 1 %s", v, RuleUnexpectedBuildTag)
	}
}

func TestT5(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "modules/server.go", "//go:build !browser\n\npackage m\n")
	writeFile(t, dir, "modules/browser.go", "//go:build browser\n\npackage m\n")
	v := VerifyLayout(dir)
	if len(v) != 0 {
		t.Errorf("VerifyLayout = %+v; want 0 violations", v)
	}
}

func TestT10(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "config/x.go", "//go:build browser && !wasm\n\npackage config\n")
	v := VerifyLayout(dir)
	if len(v) != 1 || v[0].Rule != RuleBrowserWithoutWasm {
		t.Errorf("VerifyLayout = %+v; want 1 %s", v, RuleBrowserWithoutWasm)
	}
}

func TestT11(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "modules/a/client.go", "package m\n")
	v := VerifyLayout(dir)
	if len(v) != 1 || v[0].Rule != RuleForbiddenFileName {
		t.Errorf("VerifyLayout = %+v; want 1 %s", v, RuleForbiddenFileName)
	}
}


func TestCheckWebMains_OneMainPerFolder(t *testing.T) {
	cases := []struct {
		files map[string]string
		want  string
	}{
		{
			files: map[string]string{
				"web/browser/main.go": "package main\nfunc main() {}\n",
				"web/server/main.go":  "//go:build !wasm\n\npackage main\nfunc main() {}\n",
				"web/edge/main.go":    "//go:build wasm\n\npackage main\nfunc main() {}\n",
			},
			want: "",
		},
		{
			files: map[string]string{
				"web/browser/main.go": "//go:build wasm\n\npackage main\nfunc main() {}\n",
			},
			want: "",
		},
		{
			files: map[string]string{
				"web/browser/main.go": "//go:build browser\n\npackage main\nfunc main() {}\n",
			},
			want: RuleWrongBuildTag,
		},
		{
			files: map[string]string{
				"web/server/main.go": "//go:build !browser\n\npackage main\nfunc main() {}\n",
			},
			want: RuleWrongBuildTag,
		},
		{
			files: map[string]string{
				"web/edge/main.go": "//go:build wasm && !browser\n\npackage main\nfunc main() {}\n",
			},
			want: RuleWrongBuildTag,
		},
		{
			files: map[string]string{
				"web/browser.go": "package main\nfunc main() {}\n",
			},
			want: RuleUnknownWebMain,
		},
		{
			files: map[string]string{
				"web/client.go": "package main\nfunc main() {}\n",
			},
			want: RuleUnknownWebMain,
		},
		{
			files: map[string]string{
				"web/tools/main.go": "package main\nfunc main() {}\n",
			},
			want: RuleUnknownWebMain,
		},
		{
			files: map[string]string{
				"web/public/x.go": "package main\nfunc main() {}\n",
			},
			want: "",
		},
		{
			files: map[string]string{
				"edge/main.go": "package main\nfunc main() {}\n",
			},
			want: RuleEdgeDirectory,
		},
	}

	for i, tc := range cases {
		t.Run(fmt.Sprintf("case_%d", i), func(t *testing.T) {
			dir := t.TempDir()
			for name, content := range tc.files {
				writeFile(t, dir, name, content)
			}
			v := VerifyLayout(dir)
			if tc.want == "" {
				if len(v) != 0 {
					t.Errorf("got %+v, want none", v)
				}
			} else {
				if !hasRule(v, tc.want) {
					t.Errorf("got %+v, want rule %s", v, tc.want)
				}
			}
		})
	}
}
