package notify

import "net/http"

// Option is a function used to configure the client.
type Option func(*Client) error

type httpClient interface {
	Do(*http.Request) (*http.Response, error)
}

// WithHTTPClient replaces the http.Client used for the http calls.
func WithHTTPClient(httpcli httpClient) Option {
	return func(c *Client) error {
		c.httpClient = httpcli
		return nil
	}
}

// WithUserAgent replaces the UserAgent in the http calls.
func WithUserAgent(v string) Option {
	return func(c *Client) error {
		c.userAgent = v
		return nil
	}
}

func MaxParallel(v int) Option {
	return func(c *Client) error {
		c.parallel = make(chan struct{}, v)
		return nil
	}
}
