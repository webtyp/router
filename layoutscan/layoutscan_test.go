//go:build !wasm

package layoutscan

import (
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
	writeFile(t, dir, "modules/m/server.go", "//go:build !wasm\npackage m\n")
	writeFile(t, dir, "modules/m/browser.go", "package m\n")
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

// TestR3_MissingBuildTag: modules/m/server.go without tag triggers RuleMissingBuildTag
func TestR3_MissingBuildTag(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "modules/m/server.go", "package m\n")

	v := VerifyLayout(dir)
	if !hasRule(v, RuleMissingBuildTag) {
		t.Errorf("VerifyLayout = %+v; want rule %s", v, RuleMissingBuildTag)
	}
}

// TestR3_FalsePositive: server.go with correct //go:build !wasm header and mid-file comment has 0 violations
func TestR3_FalsePositive(t *testing.T) {
	dir := t.TempDir()
	src := `//go:build !wasm
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

// TestR4_UnexpectedBuildTag: modules/m/browser.go with //go:build wasm triggers RuleUnexpectedBuildTag
func TestR4_UnexpectedBuildTag(t *testing.T) {
	dir := t.TempDir()
	src := `//go:build wasm
package m
`
	writeFile(t, dir, "modules/m/browser.go", src)

	v := VerifyLayout(dir)
	if !hasRule(v, RuleUnexpectedBuildTag) {
		t.Errorf("VerifyLayout = %+v; want rule %s", v, RuleUnexpectedBuildTag)
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

// TestR7_RegisterArity: routes/routes.go with Register(r router.Router, m ...router.APIModule) and no web/server.go triggers RuleRegisterArity
func TestR7_RegisterArity(t *testing.T) {
	dir := t.TempDir()
	src := `package routes

import "webtyp.com/router"

func Register(r router.Router, extra int) {
}
`
	writeFile(t, dir, "routes/routes.go", src)

	v := VerifyLayout(dir)
	if !hasRule(v, RuleRegisterArity) {
		t.Errorf("VerifyLayout = %+v; want rule %s", v, RuleRegisterArity)
	}
}

// TestR7_DoesNotApply: same tree WITH web/server.go produces 0 violations
func TestR7_DoesNotApply(t *testing.T) {
	dir := t.TempDir()
	src := `package routes

import "webtyp.com/router"

func Register(r router.Router, extra int) {
}
`
	writeFile(t, dir, "routes/routes.go", src)
	writeFile(t, dir, "web/server.go", "package main\n")

	v := VerifyLayout(dir)
	if len(v) != 0 {
		t.Errorf("VerifyLayout = %+v; want 0 violations", v)
	}
}

// TestAccumulation: tree with violations of R1, R2 and R5 produces 3 violations
func TestAccumulation(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "modules/m/init.go", "package m\n")       // R1
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
