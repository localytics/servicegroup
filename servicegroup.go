// Package servicegroup handles spinning up and gracefully shutting down a service by running a few linked goroutines:
// - Your service handler via an HTTP server (default at :8080)
// - The system default ServeMux with pprof enabled via an HTTP server (default at :6060) (:6060/debug/pprof)
// - Graceful shutdown routines that handles shutting both servers down
// - Sigint/sigkill listener to trigger graceful shutdown
//
// When any goroutine in the group dies or sigint/sigkill is received, the others are killed off; the HTTP servers for
// the service and pprof handler are given a timeout (default 30 seconds) to finish before being forcibly shut down.
//
// If you have other handlers you want exposed at :6060 as well (eg expvars) you can add them to the
// http default ServeMux before creating the workgroup or before calling .Run() on it.
//
// Uses heptio/workgroup to manage lifecycle of our top-level permanently-running tasks.
// Influences:
// https://dave.cheney.net/practical-go/presentations/qcon-china.html#_never_start_a_goroutine_without_knowning_when_it_will_stop
// https://github.com/pseidemann/finish
package servicegroup

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/heptio/workgroup"

	// Wire up pprof endpoints - use a separate HTTP ListenAndServe + port for this and do not wire into the app!
	// Ensure your application server is using a different port and mux than the one we'll expose below for pprof's use.
	_ "net/http/pprof"
)

// Group is a workgroup.Group that includes some server-specific configuration values. It should be constructed
// via NewGroup().
type Group struct {
	workgroup.Group
	Handler                  http.Handler  // Handler for service HTTP server
	DebugServerAddr          string        // Port for default debug server to listen on (default ":6060")
	ServiceServerAddr        string        // Port for service server (handler passed to NewGroup) to listen on (default ":8080")
	ShutdownTimeout          time.Duration // Deadline for HTTP server graceful shutdown when interrupt is sent or any worker in the Group dies
	ServiceReadHeaderTimeout time.Duration // HTTP service header read timeout (default 30 seconds). http.Server.ReadHeaderTimeout: https://golang.org/pkg/net/http/#Server
	ServiceWriteTimeout      time.Duration // HTTP timeout for all post-header-read handling, including reading body and writing response (default 30 seconds). http.Server.WriteTimeout: https://golang.org/pkg/net/http/#Server
	ServiceIdleTimeout       time.Duration // HTTP connection idle timeout (default 30 seconds). http.Server.IdleTimeout: https://golang.org/pkg/net/http/#Server
}

// NewGroup sets up http.Servers configured to use the passed handler on :8080 and debug/metrics on :6060, and an
// OS interrupt listener for graceful shutdown.
//
// Returns a servicegroup.Group that embeds a heptio/workgroup.Group ready to add more workers, or to call .Run().
//
// Additional configuration of ports and timeouts can be set *before* .Run is called by setting parameters on the
// returned Group struct. Workers and http.Servers are only initialized and started after .Run() is called.
func NewGroup(handler http.Handler) Group {
	return Group{
		Handler:                  handler,
		ShutdownTimeout:          30 * time.Second,
		ServiceReadHeaderTimeout: 30 * time.Second,
		ServiceWriteTimeout:      30 * time.Second,
		ServiceIdleTimeout:       30 * time.Second,
		DebugServerAddr:          ":6060",
		ServiceServerAddr:        ":8080",
	}
}

// Run starts the http.Servers for debug and the service using the Group's configured ports and timeouts, as
// well as any other workers you may have added to the Group.
//
// Once running, if the system gets an interrupt or any Group worker is killed, the Group's graceful-shutdown
// workers will block until they gracefully shut down the HTTP servers, with a fallback to forcibly closing the servers
// after the ShutdownTimeout period elapses.
func (g *Group) Run() error {
	log.Printf("Service starting")
	// default handlers go to :6060; for debug-type handlers.
	debugServer := &http.Server{
		Addr: g.DebugServerAddr,
		// Timeouts for debug server should be longer, but shouldn't need configurability.
		ReadHeaderTimeout: 30 * time.Second,
		WriteTimeout:      300 * time.Second,
		IdleTimeout:       30 * time.Second,
	}

	// real service handler for :8080
	serviceServer := &http.Server{
		Addr:              g.ServiceServerAddr,
		Handler:           g.Handler,
		ReadHeaderTimeout: g.ServiceReadHeaderTimeout,
		WriteTimeout:      g.ServiceWriteTimeout,
		IdleTimeout:       g.ServiceIdleTimeout,
	}

	// WORKGROUP WORKER: listen on port 6060 with default mux (pprof handler)
	// This default server should only be used for debug services and shouldn't be exposed to the public internet
	g.Add(func(stop <-chan struct{}) error {
		log.Printf("Starting debug server on %s", g.DebugServerAddr)
		return debugServer.ListenAndServe()
	})

	// WORKGROUP WORKER: gracefully shut down debug and service server on workgroup termination
	g.Add(func(stop <-chan struct{}) error {
		<-stop
		return g.shutdown(debugServer, "debug HTTP server")
	})

	// WORKGROUP WORKER: listen on port 8080 for app traffic (using the service's custom handler)
	// Real service work should happen on this custom handler, not the default debug servemux used at :6060 above.
	g.Add(func(stop <-chan struct{}) error {
		log.Printf("Starting service HTTP server on %s", g.ServiceServerAddr)
		return serviceServer.ListenAndServe()
	})

	// WORKGROUP WORKER: gracefully shut down main service server on workgroup termination
	g.Add(func(stop <-chan struct{}) error {
		<-stop
		return g.shutdown(serviceServer, "service HTTP server")
	})

	// WORKGROUP WORKER: watch for interrupt/term signals so we can shut down gracefully
	g.Add(func(stop <-chan struct{}) error {
		// interrupt/kill signals sent from terminal or host on shutdown
		interrupt := make(chan os.Signal, 1)
		signal.Notify(interrupt, syscall.SIGINT, syscall.SIGTERM)
		log.Printf("Watching for OS interrupt signals...")
		select {
		case <-stop:
			return fmt.Errorf("shutting down OS signal watcher on workgroup stop")
		case i := <-interrupt:
			log.Printf("Received OS signal %s; beginning shutdown...", i)
			return fmt.Errorf("stopping on OS signal %s", i)
		}
	})

	return g.Group.Run()
}

// Shuts down an HTTP server, using the default timeout. Attempts a graceful shutdown and then a hard close
// before returning.
func (g *Group) shutdown(server *http.Server, name string) error {
	log.Printf("Attempting graceful shutdown of %s on workgroup termination", name)
	ctx, cancel := context.WithTimeout(context.Background(), g.ShutdownTimeout)
	defer cancel()
	err := server.Shutdown(ctx)
	if err != nil {
		log.Printf("Error on graceful shutdown of %s: %s", name, err)
		log.Printf("Attempting hard shutdown of %s", name)
		err = server.Close()
		if err != nil {
			err = fmt.Errorf("error while doing hard shutdown of %s: %s", name, err)
		} else {
			err = fmt.Errorf("%s on workgroup hard shut down successful", name)
		}
	} else {
		err = fmt.Errorf("%s on workgroup graceful shut down successful", name)
	}

	log.Print(err)
	return err
}
