package main

import (
	"bytes"
	"go/build"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

const testdata = "testdata"

var origGOPATH = build.Default.GOPATH
var origWd string
var tempDir string

func init() {
	var err error
	origWd, err = os.Getwd()
	if err != nil {
		panic("couldn't get working directory: " + err.Error())
	}
}

func mkTempDir(t *testing.T) {
	var err error
	tempDir, err = ioutil.TempDir("", "go-pcp.main")
	if err != nil {
		t.Fatal(err)
	}
}

func rmTempDir(t *testing.T) {
	err := os.RemoveAll(tempDir)
	if err != nil {
		t.Fatal(err)
	}
}

func dirsEqual(dir1, dir2 string, t *testing.T) {
	filepath.Walk(dir1, func(path1 string, fi1 os.FileInfo, err error) error {
		rel, err := filepath.Rel(dir1, path1)
		if err != nil {
			t.Error("Couldn't determine relative path.")
			return nil
		}
		path2 := filepath.Join(dir2, rel)

		fi2, err := os.Stat(path2)
		if err != nil {
			t.Errorf("\"%s\" wasn't copied.", path1)
			return nil
		}

		if fi1.Mode() != fi2.Mode() {
			t.Errorf("\"%s\" wasn't copied with the correct permissions. "+
				"Expected %s and found %s.", path1, fi1.Mode(), fi2.Mode())
		}

		if fi1.IsDir() {
			return nil
		}
		b1, err := ioutil.ReadFile(path1)
		if err != nil {
			t.Errorf("Couldn't read file \"%s\": %s", path1, err)
			return nil
		}
		b2, err := ioutil.ReadFile(path2)
		if err != nil {
			t.Errorf("Couldn't read file \"%s\": %s.", path2, err)
			return nil
		}

		if !bytes.Equal(b1, b2) {
			t.Errorf("The contents of \"%s\" were not copied correctly.", path1)
		}
		return nil
	})
}

func TestThatCreateWorkspaceReturnsSyntaxErrorWhenPathIsEmpty(t *testing.T) {
	_, err := createWorkspace("")

	err = err.(syntaxErr)
	if err == nil {
		t.Error("Expected createWorkspace to return an error.")
	}
}

func TestThatCreateWorkspaceReturnsErrorWhenPathIsInvalid(t *testing.T) {
	_, err := createWorkspace("\x00")

	if err == nil {
		t.Error("Expected createWorkspace to return an error.")
	}
}

func TestThatCreateWorkspaceMakesDirectory(t *testing.T) {
	mkTempDir(t)
	defer rmTempDir(t)

	path, err := createWorkspace(filepath.Join(tempDir, "workspace"))
	if err != nil {
		t.Error("createWorkspace returned an error:", err)
	}

	_, err = os.Stat(path)
	if err != nil {
		t.Error("createWorkspace didn't create a directory:", err)
	}
}

func TestThatCreateWorkspaceReturnsPathPointingToWorkspace(t *testing.T) {
	mkTempDir(t)
	defer rmTempDir(t)

	expected := filepath.Join(tempDir, "workspace")
	path, err := createWorkspace(expected)
	if err != nil {
		t.Error("createWorkspace returned an error:", err)
	}

	expectedFile, err := os.Stat(expected)
	if err != nil {
		t.Error("Couldn't stat the expected workspace:", err)
	}
	actualFile, err := os.Stat(path)
	if err != nil {
		t.Error("Couldn't stat the actual workspace:", err)
	}
	same := os.SameFile(actualFile, expectedFile)
	if !same {
		t.Error("createWorkspace didn't return a path pointing to the " +
			"desired workspace.")
	}
}

func TestThatFindPkgReturnsPackageWithCorrectImportPath(t *testing.T) {
	importPath := "any string"
	pkg, err := findPkg(".", importPath)

	if err != nil {
		t.Error("findPkg returned an error:", err)
	}
	if pkg.ImportPath != importPath {
		t.Errorf("Package's import path didn't match the provided import path."+
			" (Expected) \"%s\" !=  (Actual) \"%s\".", importPath,
			pkg.ImportPath)
	}
}

func TestThatFindPkgReturnsPkgWhenGivenRemoteImportPath(t *testing.T) {
	_, err := findPkg("github.com/gcurtis/go-pcp", "")

	if err != nil {
		t.Error("findPkg returned an error:", err)
	}

}

func TestThatFindPkgReturnsErrorWhenPathIsInvalid(t *testing.T) {
	_, err := findPkg("\x00", "")

	if err == nil {
		t.Error("findPkg didn't return an error.")
	}
}

func TestThatFindPkgReturnsErrorWhenPathDoesNotExist(t *testing.T) {
	_, err := findPkg("/this/is/not/a/path", "")

	if err == nil {
		t.Error("findPkg didn't return an error.")
	}
}

func TestThatFindPkgReturnsErrorWhenWorkingDirectoryIsNotAPackage(t *testing.T) {
	err := os.Chdir(os.TempDir())
	if err != nil {
		t.Fatal("Couldn't change working directory:", err)
	}
	defer func() { os.Chdir(origWd) }()

	_, err = findPkg(".", "")

	if err == nil {
		t.Error("findPkg didn't return an error.")
	}
}

func TestThatCopyDirCopiesAllFilesInTest1(t *testing.T) {
	mkTempDir(t)
	defer rmTempDir(t)

	testPath := filepath.Join(testdata, "copyDir", "test1")
	dst := filepath.Join(tempDir, "dst")

	errs := copyDir(testPath, dst)

	if errs != nil {
		for _, e := range errs {
			t.Error("copyDir returned an error:", e)
		}
		t.FailNow()
	}

	dirsEqual(testPath, dst, t)
}

func TestThatCopyDirCopiesAllFilesWithCorrectPermissionsInTest2(t *testing.T) {
	mkTempDir(t)
	defer rmTempDir(t)

	testPath := filepath.Join(testdata, "copyDir", "test2")
	dst := filepath.Join(tempDir, "dst")

	errs := copyDir(testPath, dst)

	if errs != nil {
		for _, e := range errs {
			t.Error("copyDir returned an error:", e)
		}
		t.FailNow()
	}

	dirsEqual(testPath, dst, t)
}

func TestThatCopyDirCopiesAllFileContentInTest3(t *testing.T) {
	mkTempDir(t)
	defer rmTempDir(t)

	testPath := filepath.Join(testdata, "copyDir", "test3")
	dst := filepath.Join(tempDir, "dst")

	errs := copyDir(testPath, dst)

	if errs != nil {
		for _, e := range errs {
			t.Error("copyDir returned an error:", e)
		}
		t.FailNow()
	}

	dirsEqual(testPath, dst, t)
}

func TestThatCopyDirReturnsErrorWhenDestinationIsNotWriteable(t *testing.T) {
	mkTempDir(t)
	defer rmTempDir(t)

	testPath := filepath.Join(testdata, "copyDir", "test1")
	dst := filepath.Join(tempDir, "dst")

	err := os.Chmod(tempDir, 0544)
	if err != nil {
		t.Fatalf("Couldn't change the permissions of the temp dir: %s.", err)
	}
	errs := copyDir(testPath, dst)

	if errs == nil {
		t.Error("copyDir didn't return any errors.")
	}
}
