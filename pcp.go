package main

import (
	"flag"
	"fmt"
	"go/build"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var (
	hidden    bool                    // Copy hidden files.
	recursive bool                    // Recursively copy dependencies.
	abs       bool                    // Print the absolute workspace path.
	verbose   bool                    // Verbose output.
	workspace string                  // Destination workspace.
	resolved  = make(map[string]bool) // Packages that have been resolved.
	gopath    string                  // Destination GOPATH.
	exitCode  int                     // Program exit code.
)

func main() {
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: go-pcp [options] workspacePath "+
			"importPath[:directory]...")
		flag.PrintDefaults()
		fmt.Fprintln(os.Stderr, "  -help: show this message.")
		fmt.Fprintln(os.Stderr, `
go-pcp copies a Go package and its dependencies into a new Go workspace
(GOPATH). go-pcp will resolve packages by first looking in a provided directory,
then by looking for them in your existing workspace, and then by downloading
them with "go get". If go-pcp encounters an error when resolving a package, it
will skip it and continue with the next one.

Packages are specified by their import path, followed by an optional colon and
directory that contains the sources for the package. Specifying a directory lets
you use sources not in your existing Go workspace, making it useful for building
specific versions of a package (e.g., when you have a checked-out commit on a
build server). For example:

	go-pcp my-workspace github.com/gcurtis/go-pcp:$HOME/go-pcp

will use the contents of "$HOME/go-pcp" for the package "github.com/gcurtis/go-
pcp" in my-workspace.

Exit code 0 is a success, 1 is an error, 2 is a syntax error, and 3 is a
non-fatal error.`)
	}
	flag.BoolVar(&recursive, "recursive", true, "recursively copy subpackage "+
		"dependencies.")
	flag.BoolVar(&hidden, "hidden", false, "include hidden files.")
	flag.BoolVar(&abs, "abs", false, "print the absolute path to the "+
		"workspace.")
	flag.BoolVar(&verbose, "verbose", false, "verbose output.")
	flag.Parse()

	if len(flag.Args()) < 2 {
		fmt.Fprintln(os.Stderr, "At least one import is required.")
		flag.Usage()
		os.Exit(2)
	}

	var err error
	workspace, err = createWorkspace(flag.Arg(0))
	checkError(true, err)
	gopath = fmt.Sprintf("GOPATH=%s", workspace)
	if abs {
		fmt.Println(workspace)
	}

	for _, a := range flag.Args()[1:] {
		importPath, dir, err := parseImport(a)
		if checkError(false, err) {
			continue
		}

		if dir != "" {
			// Use the source found in dir for the import path.
			checkError(false, copyPkg(dir, importPath)...)
		} else {
			// No directory, look for the import path directly.
			checkError(false, copyPkg(importPath, "")...)
		}
	}

	if exitCode != 0 {
		os.Exit(exitCode)
	}
}

// parseImport takes an import string of the format "importPath[:dir]" and
// returns the separate importPath and dir components. An error is returned if
// there is a syntax error or dir could not be found.
func parseImport(s string) (importPath, dir string, err error) {
	split := strings.SplitN(s, ":", 2)
	if len(split) == 0 || split[0] == "" {
		err = syntaxErr{
			fmt: fmt.Sprintf(`"%s" isn't a valid import path.`, s),
			err: "invalid import path",
		}
		return
	}
	importPath = split[0]

	if len(split) == 2 && split[1] != "" {
		_, err = os.Stat(split[1])
		if err != nil {
			err = fmtErr{
				fmt: fmt.Sprintf(`"%s" isn't valid directory.`, split[1]),
				err: err.Error(),
			}
			return
		}
		dir, err = filepath.Abs(split[1])
		if err != nil {
			err = fmtErr{
				fmt: fmt.Sprintf(`Couldn't determine an absolute path for `+
					`"%s".`, split[1]),
				err: err.Error(),
			}
			return
		}
	}

	return
}

// checkError prints errors to stderr, optionally exiting if fatal is true and
// there is at least one error. True is returned if there was an error.
func checkError(fatal bool, errs ...error) bool {
	for _, e := range errs {
		if e == nil {
			continue
		}

		switch e := e.(type) {
		case syntaxErr:
			fmt.Fprintln(os.Stderr, e.Formatted())
			flag.Usage()
			if fatal {
				os.Exit(2)
			}
			return true
		case fmtErr:
			fmt.Fprintln(os.Stderr, e.Formatted())
			if fatal {
				os.Exit(1)
			}
			exitCode = 3
			return true
		default:
			fmt.Fprintln(os.Stderr, e)
			if fatal {
				os.Exit(1)
			}
			exitCode = 3
			return true
		}
	}

	return false
}

// createWorkspace makes a new directory at the given path and returns an
// absolute path pointing to it. A syntax error is returned if path is empty and
// any other error is returned if the directory cannot be created.
func createWorkspace(path string) (absPath string, err error) {
	if path == "" {
		err = syntaxErr{
			fmt: "You must provide a workspace path.",
			err: "path is empty",
		}
		return
	}

	absPath, err = filepath.Abs(path)
	if err != nil {
		err = fmtErr{
			fmt: fmt.Sprintf(`Couldn't determine an absolute path for "%s".`,
				path),
			err: err.Error(),
		}
		return
	}

	err = os.MkdirAll(absPath, 0744)
	if err != nil {
		err = fmtErr{
			fmt: fmt.Sprintf(`Couldn't create a directory at "%s".`, absPath),
			err: err.Error(),
		}
		return
	}

	return
}

// findPkg looks for a Go package at path and, if found, gives it an optional
// new import path. An error is returned if the package cannot be found.
//
// The path argument can be either a standard Go import path of the form
// "path/to/pkg" or it can be a directory path. Directory paths must start with
// a "." or a "/" to distinguish them from regular import paths.
func findPkg(path, newImportPath string) (pkg *build.Package, err error) {
	// Determine if path is a directory path. If it is, then we tell Go to look
	// for the import path "." in path. Otherwise, we only give Go the path,
	// causing it look in the normal GOPATH.
	srcDir := ""
	if build.IsLocalImport(path) || path[0] == '/' {
		if !filepath.IsAbs(path) {
			path, err = filepath.Abs(path)
			if err != nil {
				err = fmtErr{
					fmt: fmt.Sprintf(`Couldn't get an absolute path for "%s".`,
						path),
					err: err.Error(),
				}
				return
			}
		}

		srcDir = path
		path = "."
	}

	pkg, err = build.Import(path, srcDir, build.AllowBinary)
	if newImportPath != "" {
		pkg.ImportPath = newImportPath
	}
	if err != nil {
		var pkgStr string
		if srcDir != "" {
			pkgStr = srcDir
		} else {
			pkgStr = path
		}
		err = fmtErr{
			fmt: fmt.Sprintf(`"%s" isn't a valid Go package.`, pkgStr),
			err: fmt.Sprint("couldn't find package:", err),
		}
		return
	}

	return
}

// findSubPkgs looks for subpackages of a directory given a base path and import
// path, and returns a slice of copyPkgArgs for copying the subpackage.
//
// For example, say that "/dir/my/pkgA" with the import path "my/pkgA" has the
// subdirectoy "pkgB". findSubPkgs will return a copyPkgArg containing the path
// "dir/my/pkgA/pkgB" and the import path "my/pkgA/pkgB".
func findSubPkgs(basePath, baseImportPath string) (ret []copyPkgArgs) {
	filepath.Walk(basePath, func(path string, info os.FileInfo,
		err error) error {

		if path == basePath {
			return nil
		}
		if info.IsDir() {
			// Skip over the workspace path to prevent infinite recursion. This
			// could be cleaner by initially putting the workspace in a temp
			// directory and then moving it when done.
			if path == workspace {
				return filepath.SkipDir
			}

			rel, err := filepath.Rel(basePath, path)
			if err != nil {
				return nil
			}
			newImportPath := baseImportPath + "/" + filepath.ToSlash(rel)
			pkg, err := findPkg(path, newImportPath)
			if err != nil {
				return nil
			}
			ret = append(ret, copyPkgArgs{pkg.Dir, pkg.ImportPath})
		}
		return nil
	})
	return
}

// getPackage calls the "go get" command to download a package into a given
// GOPATH.
func getPackage(pkg string, gopath string) (err error) {
	cmd := exec.Command("go", "get", "-d", "-t", pkg)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append([]string{gopath}, os.Environ()...)
	err = cmd.Run()
	if err != nil {
		err = fmtErr{
			fmt: fmt.Sprintf(`Error downloading package "%s": %s.`, pkg, err),
			err: fmt.Sprint("error downloading package:", err),
		}
	}
	return
}

// copyPkg copies the Go package (including its dependencies) at path into the
// new workspace using an optional new import path. If path is a normal Go
// import path and newImportPath is empty, then path will be used as the new
// import path. That is, copyPkg(path, "") == copyPkg(path, path). copyPkg makes
// a best effort to keep going if an error is encountered, returning all errors
// in a slice after it finishes.
//
// copyPkg will look for packages in the following places: first in path (if it
// points to a directory), then in the preexisting GOPATH, and lastly using "go
// get". Subpackges will be recursively copied if the recursive flag is set.
// Packages that exist in the GOROOT or that have already been resolved as a
// dependency of another package will be skipped.
func copyPkg(path, newImportPath string) (errs []error) {
	pkg, err := findPkg(path, newImportPath)
	if resolved[pkg.ImportPath] {
		if verbose && !pkg.Goroot {
			fmt.Printf("Already copied %s.\n", pkg.ImportPath)
		}
		return
	}

	if err != nil || pkg.Name == "" {
		// The package could not be found, so we must download it. No need to
		// worry about dependencies or subpackages since "go get" handles that
		// for us.
		if verbose {
			fmt.Printf("Getting %s.\n", pkg.ImportPath)
		}
		err := getPackage(pkg.ImportPath, gopath)
		resolved[pkg.ImportPath] = true
		if err != nil {
			return []error{err}
		}
		return
	} else if !pkg.Goroot {
		// The package was found and doesn't exit in GOROOT, so we should copy
		// it to the new workspace.
		if verbose {
			fmt.Printf("Copying %s from \"%s\".\n", pkg.ImportPath, pkg.Dir)
		}
		dst := filepath.Join(workspace, "src", pkg.ImportPath)
		errs = copyDir(pkg.Dir, dst)
		if err != nil {
			return
		}

		// Now we must copy all of the package's dependencies by recursively
		// calling copyPkg.
		for _, dep := range pkg.Imports {
			errs = append(errs, copyPkg(dep, "")...)
		}
		resolved[pkg.ImportPath] = true

		// If the recursive flag is set, then we should copy all the subpackages
		// as well.
		if recursive {
			subPkgs := findSubPkgs(pkg.Dir, pkg.ImportPath)
			for _, sub := range subPkgs {
				if verbose {
					fmt.Printf("%s has the subpackage %s.\n", pkg.ImportPath,
						sub.newImportPath)
				}
				errs = append(errs, copyPkg(sub.path, sub.newImportPath)...)
			}
		}
	}

	return
}

// copyDir recursively copies a directory, preserving permissions. copyDir keeps
// going if an error is encountered, returning all errors in a slice after it
// finishes.
func copyDir(src, dst string) (errs []error) {
	var stack []chmod
	copyFunc := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			errs = append(errs, fmtErr{
				fmt: fmt.Sprintf(`Couldn't traverse "%s": %s.`, path, err),
				err: fmt.Sprint("error copying file:", err),
			})
			return nil
		}

		if !hidden && info.Name()[0] == '.' {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip over the workspace path to prevent infinite recursion. This
		// could be cleaner by initially putting the workspace in a temp
		// directory and then moving it when done.
		if path == workspace {
			return filepath.SkipDir
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			// This should never happen. We should always be able to figure out
			// the relative directory of the path we're walking.
			panic(fmt.Sprintf(`couldn't determine "%s" relative to "%s".`))
		}
		dstPath := filepath.Join(dst, rel)

		if info.IsDir() {
			err := os.MkdirAll(dstPath, 0744)
			if err != nil {
				errs = append(errs, fmtErr{
					fmt: fmt.Sprintf(`Couldn't create directory "%s": %s.`,
						dstPath, err),
					err: fmt.Sprintf("couldn't create dir: %s: %s", dstPath,
						err),
				})
				return nil
			}
			stack = append(stack, chmod{dstPath, info.Mode()})
			return nil
		}

		err = copyFile(path, dstPath)
		if err != nil {
			errs = append(errs, err)
		}

		stack = append(stack, chmod{dstPath, info.Mode()})
		return nil
	}

	filepath.Walk(src, copyFunc)

	// Copy permissions after file contents have been copied. We cannot copy
	// permissions when creating the copied file because the file might be read-
	// only.
	for i := len(stack) - 1; i >= 0; i-- {
		err := os.Chmod(stack[i].path, stack[i].mode)
		if err != nil {
			errs = append(errs, fmtErr{
				fmt: fmt.Sprintf(`Couldn't set permissions on "%s": %s.`,
					stack[i].path, err),
				err: fmt.Sprintf("couldn't set permissions: %s: %s",
					stack[i].path, err),
			})
		}
	}
	return
}

// copyFile copies a file, but does not preserve permissions.
func copyFile(src, dst string) error {
	fsrc, err := os.Open(src)
	if err != nil {
		return fmtErr{
			fmt: fmt.Sprintf(`Couldn't read file "%s": %s.`, src, err),
			err: fmt.Sprintf("couldn't open file for reading: %s: %s",
				src, err),
		}
	}
	defer fsrc.Close()

	fdst, err := os.Create(dst)
	if err != nil {
		return fmtErr{
			fmt: fmt.Sprintf(`Couldn't create file "%s": %s.`, dst, err),
			err: fmt.Sprintf("couldn't create file: %s: %s", dst, err),
		}
	}
	defer fdst.Close()

	_, err = io.Copy(fdst, fsrc)
	if err != nil {
		return fmtErr{
			fmt: fmt.Sprintf(`Couldn't copy the contents of "%s": %s.`, src,
				err),
			err: fmt.Sprintf("couldn't copy file: %s: %s", src, err),
		}
	}

	return nil
}

// fmtErr is an error with a formatted message suitable for printing to stderr.
type fmtErr struct{ fmt, err string }

func (f fmtErr) Formatted() string {
	return f.fmt
}

func (f fmtErr) Error() string {
	return f.err
}

// syntaxErr is an error that indicates the user entered incorrect syntax.
type syntaxErr struct{ fmt, err string }

func (s syntaxErr) Formatted() string {
	return s.fmt
}

func (s syntaxErr) Error() string {
	return s.err
}

// chmod contains a path and the path's permissions.
type chmod struct {
	path string
	mode os.FileMode
}

// copyPkgArgs contains arguments for calling copyPkg.
type copyPkgArgs struct{ path, newImportPath string }
