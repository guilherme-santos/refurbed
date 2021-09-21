package notify

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"
)

const defaultMaxParallel = 1000

type Client struct {
	notifyURL  string
	httpClient httpClient
	userAgent  string
	parallel   chan struct{}
}

func NewClient(notifyURL string, opts ...Option) *Client {
	c := &Client{
		notifyURL: notifyURL,
	}
	for _, opt := range opts {
		opt(c)
	}

	// set default values that weren't set by the options
	if c.httpClient == nil {
		// set default http client
		WithHTTPClient(http.DefaultClient)(c)
	}
	if c.parallel == nil {
		MaxParallel(defaultMaxParallel)(c)
	}

	return c
}

type Result struct {
	err   error
	errMu sync.Mutex

	// errCh receives the result of the notify operation
	errCh chan error
	// finished is closed when the whole operations is over to unblock the caller
	finished chan struct{}
}

// readError reads the error from errCh and set to err
func (r *Result) readError() {
	err := <-r.errCh

	r.errMu.Lock()
	r.err = err
	r.errMu.Unlock()

	// unblock the caller if calling Wait()
	close(r.finished)
}

// Err returns the current status of the operation, it's not guaranteed that
// the operation is over already, for that use Wait method.
func (r *Result) Err() error {
	r.errMu.Lock()
	defer r.errMu.Unlock()

	return r.err
}

// Wait waits for the operation is over and return the result of it.
func (r *Result) Wait() error {
	<-r.finished
	return r.Err()
}

func (c Client) Notify(ctx context.Context, msg string) *Result {
	// parallel has X slots (configured through MaxParallel option)
	// here we're getting one slot to be able to execute the following code.
	c.parallel <- struct{}{}

	res := Result{
		errCh:    make(chan error),
		finished: make(chan struct{}),
	}
	go res.readError()

	go func() {
		defer func() {
			// after the notification is done free up one slot for next requests.
			<-c.parallel
			close(res.errCh)
		}()

		err := ctx.Err()
		if err != nil {
			res.errCh <- err
			return
		}
		res.errCh <- c.notify(ctx, msg)
	}()

	return &res
}

// notify calls the URL provided in the NewClient using POST and the msg as body.
// If status code is different than 200, 201 or 204 an UnexpectedStatusCodeError is returned.
// Any content responded by the server will be ignored.
func (c Client) notify(ctx context.Context, msg string) error {
	body := strings.NewReader(msg)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.notifyURL, body)
	if err != nil {
		return err
	}

	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}
	// set any other necessary header, for example, we could create new Option functions
	// to set the User/Password for Basic authentication, or even a Bearer token.

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated, http.StatusNoContent:
		// all good, just ignore it
	default:
		return &UnexpectedStatusCodeError{
			StatusCode: resp.StatusCode,
		}
	}

	// discard the answer if any.
	io.Copy(ioutil.Discard, resp.Body)

	return nil
}

// UnexpectedStatusCodeError is returned when an unexpected status code is received.
type UnexpectedStatusCodeError struct {
	StatusCode int
}

func (e UnexpectedStatusCodeError) Error() string {
	return fmt.Sprintf("%d %s", e.StatusCode, http.StatusText(e.StatusCode))
}
