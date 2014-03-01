giso
====

giso is a simple tool for copying Go packages (and their dependencies) into a workspace (GOPATH). It is similar to `go get`, but differs in that it allows you to use preexisting package sources instead of downloading them. It also allows you to specify a brand new workspace, outside your normal GOPATH. This makes it easy to create isolated workspaces for testing or building a specific version of your project without messing with your normal Go workspace. It's also ideal for build servers where you want to give jobs a separate workspace to build so that they don't interfere with each other.

Note that giso is _not_ a versioning tool nor does it modify your environment. If you're looking for a tool that does these things, check out something like [gopm](https://github.com/gpmgo/gopm).

Usage
-----

```bash
$ giso -help
Usage: giso [options] workspacePath importPath[:directory]...
  -hidden=false: include hidden files.
  -recursive=true: recursively copy subpackage dependencies.
  -verbose=false: verbose output.
  -help: show this message.
```

Packages are specified by their import path, followed by an optional colon and directory that contains the sources for the package. Specifying a directory lets you use sources not in your existing Go workspace, making it useful for building specific versions of a package (e.g., when you have a checked-out commit on a build server). For example:

    giso my-workspace github.com/gcurtis/giso:$HOME/giso

will use the contents of "$HOME/giso" for the package "github.com/gcurtis/giso" in my-workspace.

Examples
--------

Here is a simple example that copies giso from my existing workspace into a new workspace located in `~/Go-Giso`:

```bash
# Assuming ~/Go is my preexisting Go workspace
$ cd ~/Go/src/github.com/gcurtis/giso
$ giso ~/Go-Giso .
...
$ GOPATH="$HOME/Go-Giso" go install
```

Since I set my GOPATH to use the new workspace before installing, my regular go workspace is not affected at all.

Here's another example that copies giso using the sources in a provided directory instead of looking in my GOPATH. This time we must give the full import path so giso knows where to put the package inside the new workspace:

```bash
$ git clone git@github.com:gcurtis/giso.git
$ giso ~/Go-Giso github.com/gcurtis/giso:giso
```

### Jenkins

This example illustrates how giso can be used with Jenkins to run Go jobs in isolation. Simply set up your job to clone your project's repo and then run the following shell script:

```bash
giso GoWorkspace github.com/gcurtis/giso:.
GOPATH=GoWorkspace
go test github.com/gcurtis/giso
go install github.com/gcurtis/giso
```

You can also archive the output by telling Jenkins to look in `GoWorkspace/bin`.

Details
-------

giso resolves packages by first looking in a provided directory (the path after the colon in `importPath[:directory]`). If the package isn't found in that directory, or a directory wasn't provided, then it looks in your preexisting GOPATH. If the package still can't be found, it is retrieved with "go get".

Once your new workspace is created, you can use it by temporarily setting it to your `GOPATH`. This is most easily done by prepending it to any Go commands (such as `GOPATH=/my/workspace go install`) or by setting it at the beginning of your build script.

If the `-recursive` flag was set (which it is by default), then subpackage dependencies will be copied as well. For example, say that your root package, PkgA, has a subpackage, PkgB, and PkgB depends on `code.google.com/p/go.net/websocket`. If you run `giso -recursive=false workspace PkgA`, then the websocket package will not be copied since it isn't a dependency of PkgA.
