package letterboxd

import (
	"bytes"
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
	baseURL  = "https://letterboxd.com"
	maxPages = 50
)

type Client struct {
	client    *http.Client
	UserAgent string
	// Config    ClientConfig
	BaseURL string
	// Options
	MaxConcurrentPages int
	Cache              *cache.Cache

	User UserService
	Film FilmService
	List ListService
	// List    ListService
	URL URLService
	// Location  LocationService
	// Volume    VolumeService
}
type ClientConfig struct {
	HTTPClient         *http.Client
	BaseURL            string
	MaxConcurrentPages int
	DisableCache       bool
	RedisHost          string
	RedisPassword      string
	RedisDB            int
	Cache              *cache.Cache
}

type Response struct {
	*http.Response
}

// NewClient Generic new client creation
func NewClient(config *ClientConfig) *Client {
	if config == nil {
		tr := &http.Transport{
			MaxIdleConns:       10,
			IdleConnTimeout:    30 * time.Second,
			DisableCompression: true,
		}
		config = &ClientConfig{
			HTTPClient: &http.Client{
				Timeout:   time.Second * 10,
				Transport: tr,
			},
			BaseURL:            baseURL,
			MaxConcurrentPages: maxPages,
		}
	}
	if config.HTTPClient == nil {
		config.HTTPClient = http.DefaultClient
	}

	userAgent := "letterrestd"
	c := &Client{
		client:    config.HTTPClient,
		UserAgent: userAgent,
		BaseURL:   baseURL,
	}

	if !config.DisableCache {
		log.Info().Msg("Configuring local cache inside client")
		if config.Cache != nil {
			c.Cache = config.Cache
		} else {
			if config.RedisHost == "" {
				log.Fatal().Msg("Cache is not disabled and no RedisHost or Cache specified")
			}
			rdb := redis.NewClient(&redis.Options{
				Addr:     config.RedisHost,
				Password: config.RedisPassword,
				DB:       config.RedisDB,
			})

			c.Cache = cache.New(&cache.Options{
				Redis:      rdb,
				LocalCache: cache.NewTinyLFU(1000, time.Minute),
			})

		}
	}

	// c.Location = &LocationServiceOp{client: c}
	// c.Volume = &VolumeServiceOp{client: c}
	c.User = &UserServiceOp{client: c}
	c.Film = &FilmServiceOp{client: c}
	// c.List = &ListServiceOp{client: c}
	c.URL = &URLServiceOp{client: c}
	c.List = &ListServiceOp{client: c}
	return c
}

type PageData struct {
	Data      interface{}
	Pagintion Pagination
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

func (c *Client) sendRequest(req *http.Request, extractor func(io.Reader) (interface{}, *Pagination, error)) (*PageData, *Response, error) {
	res, err := c.client.Do(req)
	req.Close = true
	if err != nil {
		log.Warn().Err(err).Msg("Error sending request")
		return nil, nil, err
	}
	defer res.Body.Close()

	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusBadRequest {
		var errRes ErrorResponse
		// b, _ := ioutil.ReadAll(res.Body)
		// log.Debug(string(b))
		if err = json.NewDecoder(res.Body).Decode(&errRes); err == nil {
			return nil, nil, errors.New(errRes.Message)
		}

		if res.StatusCode == http.StatusTooManyRequests {
			return nil, nil, fmt.Errorf("too many requests.  Check rate limit and make sure the userAgent is set right")
		} else if res.StatusCode == http.StatusNotFound {
			log.Warn().
				Int("status", res.StatusCode).
				Str("url", req.URL.String()).
				Msg("Not found")
			return nil, nil, fmt.Errorf("that entry was not found, are you sure it exists?")
		} else {
			return nil, nil, fmt.Errorf("error, status code: %d", res.StatusCode)
		}
	}

	b, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, nil, err
	}
	items, pagination, err := extractor(bytes.NewReader(b))
	if err != nil {
		log.Warn().Msg("Error parsing response")
		return nil, nil, err
	}
	r := &Response{res}
	d := &PageData{
		Data: items,
	}
	if pagination != nil {
		d.Pagintion = *pagination
	}

	return d, r, nil
}

type ErrorResponse struct {
	Message string `json:"errors"`
}

// MustNewRequest is a wrapper around http.NewRequest that panics if an error
// occurs
func MustNewRequest(method, url string, body io.Reader) *http.Request {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		panic(err)
	}
	return req
}
