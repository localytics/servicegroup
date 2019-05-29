# localytics/servicegroup

Golang HTTP server spin-up with best-practices safety in timeouts, debug endpoints, and graceful shutdown.

Go ships with a production-ready webserver you can spin up [in a few lines of code](https://youtu.be/rFejpH_tAHM?t=1328)… right? Well, it's close.

Servicegroup spins up a `net/http` server just as easily, but sets up:

* Sensible [timeouts](https://blog.cloudflare.com/the-complete-guide-to-golang-net-http-timeouts/) and keepalives.
* Debug endpoints (/debug/pprof) but on a different server/port (:6060 by default).
* Graceful shutdown signal (ctrl+c/SIGINT, SIGKILL) handling without interrupting in-flight requests/responses.

This avoids the risks of slow requests DOSing your service, leaking debug info on public ports/endpoints, or normal server shutdowns leading to broken client requests.

Just bring your own http.Handler (from a basic `http.ServeMux` up to something like [chi.Router](https://github.com/go-chi/chi)) and wire it in:

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

Servicegroup embeds a [heptio workgroup](https://github.com/heptio/workgroup). 

If you have other goroutines you would like to mutually anchor to the lifecycle of the servicegroup (metrics reporters, loggers, background cleanup tasks, etc.), just add them with `.Add()`; see the [heptio workgroup](https://github.com/heptio/workgroup) docs for details. 

### Running The Example Server

```
go run cmd/servicegroup-example-server/main.go
```

Then visit `localhost:8080/work` in a browser. Hit ctrl+c on the running server while the page is loading to see a graceful shutdown in action.

Or run the example in Docker:

```sh
docker build . -t servicegroup-example:latest
docker run --rm -p 8080:8080 -p 6060:6060 servicegroup-example:latest
```

## Contributing & Development

### Tests

You can use the provided `go` docker-compose service via `docker-compose run` to run one-offs in a container.
Tests only:

```sh
docker-compose run --rm go test -v ./...
```

The full test suite and linters using the CI scripts:

```sh
docker-compose run --rm --entrypoint ci/checks.sh go
```

Note that if you use `go run` in Compose to run the servicegroup-example-server, SIGKILL won't propagate; you'll need to run a docker image with a built binary or test shutdown via SIGINT instead.

### Code Style & Formatting

`goimports` is `gofmt`, but better. Set your editor up to run it on file save.
