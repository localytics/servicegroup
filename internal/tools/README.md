# internal/tools

Module-managed tool-only dependencies go here. We use a separate module with a different name so we don't leak tools to packages that just import this as a dependency. Uses [gex](https://github.com/izumin5210/gex); see the project root `install_tools.sh` for usage.

To add a new tool run `gex add` in this directory (not the project root!); for example:

```sh
gex add github.com/cespare/reflex@v0.2.0
```

This will modify the `tools.go` file and update the `go.mod`/`go.sum` files appropriately.
