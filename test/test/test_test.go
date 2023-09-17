package test

import (
	"os/exec"
	"testing"
)

func TestSourcemap(t *testing.T) {
	execUserCase("sourcemap", t)
}

func TestSassImport(t *testing.T) {
	execUserCase("sass_import", t)
}

func execUserCase(userCase string, t *testing.T) {
	modTidy := exec.Command("go", "mod", "tidy")
	run := exec.Command("go", "run", "main.go")
	modTidy.Dir = "../" + userCase
	run.Dir = "../" + userCase
	err := modTidy.Run()
	if err != nil {
		t.Fatalf("err: %s", err.Error())
		return
	}
	err = run.Run()
	if err != nil {
		t.Fatalf("err: %s", err.Error())
		return
	}
}

func snapshotCompare(snapshotDir string, ouptDir string, ignoreFiles []string) {

}
