package notify_test

import (
	"bytes"
	"context"
	"errors"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

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
	err := res.Wait()
	if err != nil {
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
	err := res.Wait()
	if err != context.Canceled {
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
	err := res.Wait()

	var expErr *notify.UnexpectedStatusCodeError
	if errors.As(err, &expErr) {
		if expErr.StatusCode != http.StatusInternalServerError {
			assert(t, http.StatusInternalServerError, expErr.StatusCode)
		}
	} else {
		t.Errorf("expected %T but received %T", expErr, err)
	}
}

func assert(t *testing.T, expected, received interface{}) {
	t.Helper()
	t.Errorf("expected %q but received %q", expected, received)
}
