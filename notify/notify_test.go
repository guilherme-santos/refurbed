package notify_test

import (
	"bytes"
	"context"
	"errors"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"
	"time"

	"github.com/guilherme-santos/refurbed/notify"
)

const testMessage = "my message goes here"

func TestNotify(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPost {
			assert(t, http.MethodPost, req.Method)
			return
		}
		body, _ := ioutil.ReadAll(req.Body)
		if !bytes.Equal([]byte(testMessage), body) {
			assert(t, testMessage, string(body))
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	ctx := context.Background()

	c := notify.NewClient(srv.URL)
	res := c.Notify(ctx, testMessage)

	res.Wait()

	if err := res.Err(); err != nil {
		t.Errorf("no error was expected but received: %v", err)
	}
}

func TestNotify_WithCanceledContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		t.Error("unexpected http call")
	}))
	defer srv.Close()

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	cancel() // cancel before call Notify

	c := notify.NewClient(srv.URL)
	res := c.Notify(ctx, testMessage)

	res.Wait()

	if err := res.Err(); err != context.Canceled {
		assert(t, context.Canceled, err)
		return
	}
}

func TestNotify_UnexpectedStatusCode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	ctx := context.Background()

	c := notify.NewClient(srv.URL)
	res := c.Notify(ctx, testMessage)

	res.Wait()
	err := res.Err()

	var expErr *notify.UnexpectedStatusCodeError
	if errors.As(err, &expErr) {
		if expErr.StatusCode != http.StatusInternalServerError {
			assert(t, http.StatusInternalServerError, expErr.StatusCode)
		}
	} else {
		t.Errorf("expected %T but received %T", expErr, err)
	}
}

// TestNotify_WorksInParallel makes the message_1 slower and check if message_2 ends first.
func TestNotify_WorksInParallel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		body, _ := ioutil.ReadAll(req.Body)
		msg := string(body)

		if msg == "message_1" {
			time.Sleep(500 * time.Millisecond)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	ctx := context.Background()

	c := notify.NewClient(srv.URL)

	firstRequest := make(chan struct{})
	secondRequest := make(chan struct{})

	res1 := c.Notify(ctx, "message_1")
	// Give change for the first request goes though
	runtime.Gosched()

	res2 := c.Notify(ctx, "message_2")

	go func() {
		res1.Wait()
		close(firstRequest)
	}()
	go func() {
		res2.Wait()
		close(secondRequest)
	}()

	select {
	case <-firstRequest:
		t.Fatal("first request ended first but it's slower, parallelism is not working")
	case <-secondRequest:
	}
}

// TestNotify_MaxParallelThrottle makes the message_1 slower and check if even though
// message_2 will end last.
func TestNotify_MaxParallelThrottle(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		body, _ := ioutil.ReadAll(req.Body)
		msg := string(body)

		if msg == "message_1" {
			time.Sleep(500 * time.Millisecond)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	ctx := context.Background()

	c := notify.NewClient(srv.URL, notify.MaxParallel(1))

	firstRequest := make(chan struct{})
	secondRequest := make(chan struct{})

	res1 := c.Notify(ctx, "message_1")
	res2 := c.Notify(ctx, "message_2")

	go func() {
		res1.Wait()
		close(firstRequest)
	}()
	go func() {
		res2.Wait()
		close(secondRequest)
	}()

	select {
	case <-firstRequest:
	case <-secondRequest:
		t.Fatal("second request ended first, throttle didn't work")
	}
}

func assert(t *testing.T, expected, received interface{}) {
	t.Helper()
	t.Errorf("expected %q but received %q", expected, received)
}
