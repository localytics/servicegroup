# localytics/servicegroup

Golang HTTP server spin-up with best-practices safety in timeouts, debug endpoints, and graceful shutdown.

Go ships with a production-ready webserver you can spin up [in a few lines of code](https://youtu.be/rFejpH_tAHM?t=1328)… right? Well, it's close.

Servicegroup spins up a `net/http` server just as easily, but sets up:

* Sensible [timeouts](https://blog.cloudflare.com/the-complete-guide-to-golang-net-http-timeouts/) and keepalives.
* [pprof debugging endpoints](https://golang.org/pkg/net/http/pprof/) on a different server/port (:6060 by default).
* Graceful shutdown signal handling (ctrl+c/`SIGINT`, `SIGKILL`) without interrupting in-flight requests/responses.

This avoids the risks of slow requests DOSing your service, leaking debug info on public ports/endpoints, or normal server shutdowns leading to broken client requests.

Just bring your own `http.Handler` (from a basic `http.ServeMux` up to something like [`chi.Router`](https://github.com/go-chi/chi)) and wire it in:

```go
mux := http.NewServeMux()
mux.HandleFunc("/ping", func(w http.ResponseWriter, req *http.Request) {fmt.Fprintf(w, "pong")}) 
group := servicegroup.NewGroup(mux) // the main server (:8080 by default) binds to the handler/mux passed in here
// Run() starts the servicegroup synchronously, and returns the error that terminated the group when it shuts down.
err := group.Run()
log.Printf("Servicegroup terminated due to initial worker termination: %s", err)
```

You can configure timeouts and ports by modifying the returned `Group` struct's fields before calling `.Run()`.

Note that servicegroup does not handle TLS; the assumption is you're using this behind a load balancer or gateway that terminates SSL.

## Example & Docs

You can [view the godoc online](https://godoc.org/github.com/localytics/servicegroup).

See [cmd/servicegroup-example-server](https://github.com/localytics/servicegroup/tree/master/cmd/servicegroup-example-server/main.go) for a runnable example. 
  
## How it Works

Servicegroup embeds a [Heptio workgroup](https://github.com/heptio/workgroup). It runs the servers, signal handler, and graceful shutdown calls in separate goroutines under the hood, using channels managed by the underlying Heptio workgroup to coordinate.
 
The main HTTP handler runs in an http.Server at :8080 by default. The debug endpoints use the default ServeMux running in a separate `http.Server` bound to :6060 by default.

When an interrupt/kill signal is received or any of the goroutines terminate, Servicegroup calls the graceful [Shutdown()](https://golang.org/pkg/net/http/#Server.Shutdown) on both servers; if the Shutdown call exceeds a timeout, that server's [Close()](https://golang.org/pkg/net/http/#Server.Close) is called to force shutdown.

If you have other permanently-running tasks you want to mutually anchor to the lifecycle of the servicegroup (metrics reporters, loggers, background cleanup tasks, etc.), you can add them with `.Add()`; see the [heptio workgroup](https://github.com/heptio/workgroup) docs for details. 

### Running The Example Server

```bash
go run cmd/servicegroup-example-server/main.go
```

Then visit `localhost:8080/work` in a browser. Hit ctrl+c on the running server while the page is loading to see a graceful shutdown in action.
`localhost:6060/debug/pprof` has [the standard pprof goods](https://matoski.com/article/golang-profiling-flamegraphs/); visit it directly or run `go tool pprof` commands against it.

You can also run the example in Docker:

```bash
docker build . -t servicegroup-example:latest
docker run --rm -p 8080:8080 -p 6060:6060 servicegroup-example:latest
```

## Contributing & Development

### Tests

You can use the provided `go` docker-compose service via `docker-compose run` to run one-offs in a container.
Tests only:

```bash
docker-compose run --rm go test -v ./...
```

The full test suite and linters using the CI scripts:

```bash
docker-compose run --rm --entrypoint ci/checks.sh go
```

Note that if you use `go run` in Compose to run the servicegroup-example-server, SIGKILL won't propagate; you'll need to run a Docker image with a built binary or test shutdown via SIGINT instead.

### Code Style & Formatting

`goimports` is `gofmt`, but better. Set your editor up to run it on file save.
