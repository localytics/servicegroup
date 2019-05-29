package main

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/localytics/servicegroup"
)

// You can add more things to the default servemux too, which will all end up at :6060 along with the
// debug/pprof endpoints we wire in automatically. For example, this adds /debug/vars from expvars to :6060.
import _ "expvar"

func main() {
	// Use a separate mux for our service at :8080 (the default Go ServeMux is used for :6060/debug/pprof).
	mux := http.NewServeMux() // This can be any handler; a full-fledged router like Chi, for example.
	mux.HandleFunc("/work", work)
	group := servicegroup.NewGroup(mux)
	// You can configure the group before starting it:
	group.ShutdownTimeout = 10 * time.Second
	// Run() starts the servicegroup:
	err := group.Run()
	log.Printf("Servicegroup terminated due to initial worker termination: %s", err)
}

// Mimic a slow request that takes some time to complete - notice that sending ctrl-c while a request is pending allows
// the request to complete before shutting down.
func work(w http.ResponseWriter, req *http.Request) {
	log.Print("starting request…")
	rand.Seed(time.Now().UnixNano())
	sleepTimeMillis := rand.Intn(5000) + 5000
	time.Sleep(time.Duration(sleepTimeMillis) * time.Millisecond)
	fmt.Fprintf(w, "that took %d ms", sleepTimeMillis)
	log.Printf("request finished in %d ms", sleepTimeMillis)
}
