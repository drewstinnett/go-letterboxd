/*
Package letterboxd is the client for interacting with the http api
*/
package letterboxd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/go-redis/cache/v8"
	"github.com/go-redis/redis/v8"
	"github.com/rs/zerolog/log"
)

const (
	baseURL   = "https://letterboxd.com"
	maxPages  = 50
	userAgent = "letterrestd"
)

// Client represents the thing containing services and methods for interacting with Letterboxd
type Client struct {
	client    *http.Client
	UserAgent string
	// Config    ClientConfig
	baseURL string
	// Options
	MaxConcurrentPages int
	Cache              *cache.Cache

	User UserService
	Film FilmService
	List ListService
	URL  URLService
}

// ClientConfig is the configuration strcut for the client
type ClientConfig struct {
	HTTPClient         *http.Client
	BaseURL            string
	MaxConcurrentPages int
	DisableCache       bool
	RedisHost          string
	RedisPassword      string
	RedisDB            int
	Cache              *cache.Cache
	CacheTime          *time.Duration
	// Maybe favor this instead of cache.Cache?
	RedisClient *redis.Client
}

// Response holds the http response and metadata arounda given request
type Response struct {
	*http.Response
	FromCache bool
	// pagination *Pagination
}

// WithCache applies a given cache.Cache to the letterboxd library
func WithCache(cc *cache.Cache) func(*Client) {
	return func(c *Client) {
		c.Cache = cc
	}
}

// WithNoCache removes the default cache
func WithNoCache() func(*Client) {
	return func(c *Client) {
		c.Cache = nil
	}
}

// WithBaseURL sets the url (Example: https://letterboxd.com) to use for scraping
func WithBaseURL(u string) func(*Client) {
	return func(c *Client) {
		c.baseURL = u
	}
}

// New returns a new client using functional options
func New(options ...func(*Client)) *Client {
	// Set up some sane defaults
	c := &Client{
		client: &http.Client{
			Timeout: time.Second * 10,
			Transport: &http.Transport{
				MaxIdleConns:       10,
				IdleConnTimeout:    30 * time.Second,
				DisableCompression: true,
			},
		},
		UserAgent:          userAgent,
		baseURL:            baseURL,
		MaxConcurrentPages: maxPages,
		Cache: cache.New(&cache.Options{
			Redis: redis.NewClient(&redis.Options{
				Addr: "127.0.0.1:6379",
			}),
			LocalCache: cache.NewTinyLFU(1000, time.Minute),
		}),
	}

	// Apply all the options
	for _, o := range options {
		o(c)
	}

	c.User = &UserServiceOp{client: c}
	c.Film = &FilmServiceOp{client: c}
	c.URL = &URLServiceOp{client: c}
	c.List = &ListServiceOp{client: c}
	return c
}

// PageData just provides Pagination info and 'Data'
type PageData struct {
	Data       interface{}
	Pagination Pagination
}

/*
type ThrottledTransport struct {
	roundTripperWrap http.RoundTripper
	ratelimiter      *rate.Limiter
}

func (c *ThrottledTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	err := c.ratelimiter.Wait(r.Context()) // This is a blocking call. Honors the rate limit
	if err != nil {
		return nil, err
	}
	return c.roundTripperWrap.RoundTrip(r)
}
*/

// https://gist.github.com/zdebra/10f0e284c4672e99f0cb767298f20c11
// NewThrottledTransport wraps transportWrap with a rate limitter
// examle usage:
// client := http.DefaultClient
// client.Transport = NewThrottledTransport(10*time.Seconds, 60, http.DefaultTransport) allows 60 requests every 10 seconds
/*
func NewThrottledTransport(limitPeriod time.Duration, requestCount int, transportWrap http.RoundTripper) http.RoundTripper {
	return &ThrottledTransport{
		roundTripperWrap: transportWrap,
		ratelimiter:      rate.NewLimiter(rate.Every(limitPeriod), requestCount),
	}
}
*/

func (c *Client) getFromCache(ctx context.Context, key string) *PageData {
	var pData *PageData
	if c.Cache != nil {
		if err := c.Cache.Get(ctx, key, pData); err == nil {
			return pData
		}
	}
	return nil
}

func (c *Client) setCache(ctx context.Context, key string, pData PageData) {
	if c.Cache != nil {
		// max, min := 72, 24
		// cacheFor := rand.Intn(max-min) + min // nolint:golint,gosec
		if err := c.Cache.Set(&cache.Item{
			Ctx:   ctx,
			Key:   key,
			Value: pData,
			TTL:   time.Hour * 24,
		}); err != nil {
			log.Warn().Err(err).Msg("Error Writing Cache")
		}
	}
}

// checkResponse is just a little helper to see if an http.Response is good or not
func checkResponse(res *http.Response) error {
	// func (c *Client) checkResponse(res *http.Response) error {
	var err error
	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusBadRequest {
		var errRes ErrorResponse
		if err = json.NewDecoder(res.Body).Decode(&errRes); err == nil {
			return errors.New(errRes.Message)
		}

		switch {
		case res.StatusCode == http.StatusTooManyRequests:
			return fmt.Errorf("too many requests.  Check rate limit and make sure the userAgent is set right")
		case res.StatusCode == http.StatusNotFound:
			return fmt.Errorf("that entry was not found, are you sure it exists?")
		default:
			return fmt.Errorf("error, status code: %d", res.StatusCode)
		}
	}
	return nil
}

func (c *Client) sendRequest(req *http.Request, extractor func(io.Reader) (interface{}, *Pagination, error)) (*PageData, *Response, error) {
	key := fmt.Sprintf("/letterboxd/fullpage%s", req.URL.Path)

	// Do we have this page cached?
	pData := c.getFromCache(context.TODO(), key)
	// Did we get an actual PageData back, or just nil?
	if pData == nil {
		res, err := c.client.Do(req)
		req.Close = true
		if err != nil {
			return nil, nil, err
		}
		defer dclose(res.Body)

		err = checkResponse(res)
		if err != nil {
			return nil, nil, err
		}

		b, err := io.ReadAll(res.Body)
		if err != nil {
			return nil, nil, err
		}
		if string(b) == "" {
			log.Warn().
				Int("status", res.StatusCode).
				Str("url", req.URL.String()).
				Msg("Empty body found. Check reader...")
		}
		items, pagination, err := extractor(bytes.NewReader(b))
		if err != nil {
			return nil, nil, err
		}
		// log.Warn().Interface("send-pagination", pagination).Send()
		d := &PageData{
			Data: items,
		}
		if pagination != nil {
			d.Pagination = *pagination
		}

		// Save to cache before returning
		c.setCache(context.TODO(), key, *d)

		return d, &Response{
			Response:  res,
			FromCache: false,
		}, nil
	}
	return pData, &Response{
		FromCache: true,
	}, nil
}

// ErrorResponse just contains the errors of a response
type ErrorResponse struct {
	Message string `json:"errors"`
}

// mustNewRequest is a wrapper around http.NewRequest that panics if an error
// occurs
func mustNewRequest(method, url string, body io.Reader) *http.Request {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		panic(err)
	}
	return req
}

// mustNewGetRequest is a simple wrapper to do a get request with no body
func mustNewGetRequest(url string) *http.Request {
	return mustNewRequest("GET", url, nil)
}

func dclose(c io.Closer) {
	if err := c.Close(); err != nil {
		panic(err)
	}
}

/*
func mustParseURL(path string) *url.URL {
	u, err := url.Parse(path)
	if err != nil {
		panic(err)
	}
	return u
}
*/
