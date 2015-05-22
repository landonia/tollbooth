// Package tollbooth provides rate-limiting logic to HTTP request handler.
package tollbooth

import (
	"github.com/landonia/tollbooth/config"
	"github.com/landonia/tollbooth/errors"
	"github.com/landonia/tollbooth/libstring"
	"net/http"
	"strings"
	"time"
)

// NewLimiter is a convenience function to config.NewLimiter.
func NewLimiter(max int64, ttl time.Duration) *config.Limiter {
	return config.NewLimiter(max, ttl)
}

// LimitByKeys keeps track number of request made by keys separated by pipe.
// It returns HTTPError when limit is exceeded.
func LimitByKeys(limiter *config.Limiter, keys []string) *errors.HTTPError {
	if limiter.LimitReached(strings.Join(keys, "|")) {
		return &errors.HTTPError{Message: limiter.Message, StatusCode: limiter.StatusCode}
	}

	return nil
}

func buildKeys(limiter *config.Limiter, r *http.Request) [][]string {
	remoteIP := libstring.RemoteIP(r)
	path := r.URL.Path
	sliceKeys := make([][]string, 0)

	if limiter.Methods != nil && limiter.Headers != nil {
		// Limit by HTTP methods and HTTP headers+values.
		if libstring.StringInSlice(limiter.Methods, r.Method) {
			for headerKey, headerValues := range limiter.Headers {
				if (headerValues == nil || len(headerValues) <= 0) && r.Header.Get(headerKey) != "" {
					// If header values are empty, rate-limit all request with headerKey.
					sliceKeys = append(sliceKeys, []string{remoteIP, path, r.Method, headerKey})

				} else if len(headerValues) > 0 && r.Header.Get(headerKey) != "" {
					// If header values are not empty, rate-limit all request with headerKey and headerValues.
					for _, headerValue := range headerValues {
						sliceKeys = append(sliceKeys, []string{remoteIP, path, r.Method, headerKey, headerValue})
					}
				}
			}
		}

	} else if limiter.Methods != nil {
		// Limit by HTTP methods.
		if libstring.StringInSlice(limiter.Methods, r.Method) {
			sliceKeys = append(sliceKeys, []string{remoteIP, path, r.Method})
		}

	} else if limiter.Headers != nil {
		// Limit by HTTP headers+values.
		for headerKey, headerValues := range limiter.Headers {
			if (headerValues == nil || len(headerValues) <= 0) && r.Header.Get(headerKey) != "" {
				// If header values are empty, rate-limit all request with headerKey.
				sliceKeys = append(sliceKeys, []string{remoteIP, path, headerKey})

			} else if len(headerValues) > 0 && r.Header.Get(headerKey) != "" {
				// If header values are not empty, rate-limit all request with headerKey and headerValues.
				for _, headerValue := range headerValues {
					sliceKeys = append(sliceKeys, []string{remoteIP, path, headerKey, headerValue})
				}
			}
		}

	} else {
		// Default: Limit by remoteIP and path.
		sliceKeys = append(sliceKeys, []string{remoteIP, path})
	}

	return sliceKeys
}

// LimitHandler is a middleware that performs rate-limiting given http.Handler struct.
func LimitHandler(limiter *config.Limiter, next http.Handler) http.Handler {
	middle := func(w http.ResponseWriter, r *http.Request) {
		sliceKeys := buildKeys(limiter, r)

		var httpError *errors.HTTPError

		// Loop sliceKeys and check if one of them has error.
		for _, keys := range sliceKeys {
			httpError = LimitByKeys(limiter, keys)
			if httpError != nil {
				http.Error(w, httpError.Message, httpError.StatusCode)
				return
			}
		}

		// There's no rate-limit error, serve the next handler.
		next.ServeHTTP(w, r)
	}
	return http.HandlerFunc(middle)
}

// LimitFuncHandler is a middleware that performs rate-limiting given request handler function.
func LimitFuncHandler(limiter *config.Limiter, nextFunc func(http.ResponseWriter, *http.Request)) http.Handler {
	return LimitHandler(limiter, http.HandlerFunc(nextFunc))
}
