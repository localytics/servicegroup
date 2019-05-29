package servicegroup

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestNewWorkgroup_ShutsDownGracefully(t *testing.T) {
	// * Spin up a service with a ping plus a slow "work" endpoint
	// * When it comes up, make a "work" request then immediately send an interrupt
	// * Validate that the "work" request gets a response before the server shuts down, and that shutdown took a
	//   reasonable amount of time
	workDuration := time.Duration(100) * time.Millisecond

	mux := http.NewServeMux() // custom mux for our service (:8080)
	mux.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "pong")
	})
	mux.HandleFunc("/work", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(workDuration)
		fmt.Fprintf(w, "that took %v", workDuration)
	})
	group := NewGroup(mux)

	process, err := os.FindProcess(os.Getpid())
	Ok(t, err)
	fmt.Printf("process is %v", process.Pid)

	workResponseBody := make(chan string)

	// Wait until the server is available, then make a long-lived work request, send sigint, and send the work request
	// results back on a channel.
	go func() {
		timeout := time.After(3 * time.Second)
		for {
			select {
			case <-timeout:
				Assert(t, false, "Timed out waiting for test server to become available.")
				return
			default:
				resp, err := http.Get("http://127.0.0.1:8080/ping")
				if err == nil {
					resp.Body.Close()
					Ok(t, err)
					// Start a request to the slow endpoint in the background & send a sigint
					go func() {
						resp, err := http.Get("http://127.0.0.1:8080/work")
						Ok(t, err)
						body, err := ioutil.ReadAll(resp.Body)
						resp.Body.Close()
						Ok(t, err)
						workResponseBody <- string(body)
					}()
					// Give the goroutine a beat to make the request, then ctrl+c
					time.Sleep(time.Duration(1) * time.Millisecond)
					err := process.Signal(syscall.SIGINT)
					Ok(t, err)
					return
				}
			}
		}
	}()

	startTime := time.Now()
	err = group.Run()
	select {
	case body := <-workResponseBody:
		Assert(t, time.Since(startTime) < time.Second*5, "Exceeded expected shutdown timing")
		Assert(t, strings.HasPrefix(body, "that took"), "response body must match expected value")
	default:
		Assert(t, false, "No response body received before server shutdown. Group shutdown root error: %s", err)
	}
}

// Test helpers for common tasks that don't require leaking heavy test libraries as module
// dependencies to consumers. Slight variation of https://github.com/benbjohnson/testing

// Assert fails the test if the condition is false.
func Assert(tb testing.TB, condition bool, msg string, v ...interface{}) {
	if !condition {
		_, file, line, _ := runtime.Caller(1)
		fmt.Printf("\033[31m%s:%d: "+msg+"\033[39m\n\n", append([]interface{}{filepath.Base(file), line}, v...)...)
		tb.FailNow()
	}
}

// Ok fails the test if an err is not nil.
// msgAndArgs is an optional string to print on failure (supporting formatting)
// followed by optional format args.
func Ok(tb testing.TB, err error, msgAndArgs ...interface{}) {
	if err != nil {
		_, file, line, _ := runtime.Caller(1)
		fmt.Printf("\033[31m%s:%d: ", filepath.Base(file), line)
		if len(msgAndArgs) > 0 {
			fmt.Printf(msgAndArgs[0].(string)+"\n\t", msgAndArgs[1:]...)
		}
		fmt.Printf("unexpected error: %s\033[39m\n\n", err.Error())
		tb.FailNow()
	}
}

// Equals fails the test if exp is not equal to act.
// msgAndArgs is an optional string to print on failure (supporting formatting)
// followed by optional format args.
func Equals(tb testing.TB, exp, act interface{}, msgAndArgs ...interface{}) {
	if !reflect.DeepEqual(exp, act) {
		_, file, line, _ := runtime.Caller(1)
		fmt.Printf("\033[31m%s:%d: ", filepath.Base(file), line)
		if len(msgAndArgs) > 0 {
			fmt.Printf(msgAndArgs[0].(string)+"\n\t", msgAndArgs[1:]...)
		}
		fmt.Printf("exp: %#v\n\n\tgot: %#v\033[39m\n\n", exp, act)
		tb.FailNow()
	}
}

// Don't warn on unused helpers
var _, _, _ = Assert, Ok, Equals
