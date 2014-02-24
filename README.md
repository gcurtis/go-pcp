giso
====

giso is a simple tool for isolating your Go projects into a new workspace (`GOPATH`). It allows you to easily test or build a specific version of your project without interfering with your normal Go workspace. This makes it ideal for continuous integration where you want to give jobs a separate workspace so that they don't interfere with each other.

Note that giso is _not_ a versioning tool nor does it modify your environment. If you're looking for a tool that does these things, check out something like [gopm](https://github.com/gpmgo/gopm).

Usage
-----

giso will copy your project into a new workspace along with all of its dependencies (excluding standard Go packages). Here is a simple example that isolates giso into a new workspace located in `~/Go-Giso`:

```bash
$ cd ~/Go/src/github.com/gcurtis/giso
$ giso "~/Go-Giso"
...
$ GOPATH="$HOME/Go-Giso" go install
```

Since I set my `GOPATH` to use the new workspace before installing, my regular go workspace is not affected at all.

```bash
$ giso -help
Usage: giso [options] <new-workspace> [import-path]
    -project=".": project directory.
    -help: show this message.
```

### Details

giso first looks at your project directory. If your project is already in a Go workspace, it will automatically be copied over into the new workspace. If your project isn't already in a Go workspace, you will need to tell giso the import path of your project. For example:

```bash
$ cd ~/Go/src/github.com/gcurtis/giso
$ giso "~/Go-Giso"
```

```bash
$ cd ~/Desktop/giso
$ giso "~/Go-Giso" "github.com/gcurtis/giso"
```

After your project is copied, giso will look at its dependencies and begin copying them into the new workspace. If a dependency already exists in your GOPATH, it will simply be copied over. This can give you some _very_ coarse-grain control over the versions of your project's dependencies. Simply checkout the version of the dependency you want to use, and that's the one giso will copy. If the dependency cannot be found, giso will download it using `go get`.

Once your new workspace is created, you can use it by temporarily setting it to your `GOPATH`. This is most easily done by prepending it to any Go commands (such as `GOPATH=/my/workspace go install`) or by setting it at the beginning of your build script.

### Jenkins Example

This example illustrates how giso can be used with Jenkins to run Go jobs in isolation. Simply set up your job to clone your project's repo and then run the following shell script:

```bash
giso "Go Workspace"
GOPATH="./Go Workspace"
go test github.com/gcurtis/giso
go install github.com/gcurtis/giso
```

You can also archive the output by telling Jenkins to look in `Go Workspace/bin`.
