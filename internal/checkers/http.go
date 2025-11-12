package checkers

import (
	"net/http"
	"net/url"
	"time"
)

type SimpleHTTPChecker struct {
	URL string
}

func NewSimpleHTTPChecker(url string) *SimpleHTTPChecker {
	return &SimpleHTTPChecker{
		URL: url,
	}
}

func (c *SimpleHTTPChecker) CheckHealth() (bool, string, error) {
	const maxRetries = 3
	const retryDelay = 10 * time.Second

	// Validate URL before attempting request
	_, err := url.Parse(c.URL)
	if err != nil {
		return false, "", err
	}

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	var lastStatus string

	for attempt := 1; attempt <= maxRetries; attempt++ {
		resp, err := client.Get(c.URL)
		if err != nil {
			lastStatus = err.Error()

			// If this is not the last attempt, wait before retrying
			if attempt < maxRetries {
				time.Sleep(retryDelay)
				continue
			}

			// Connection errors mean service is unhealthy, not an internal error
			return false, lastStatus, nil
		}
		defer resp.Body.Close() //nolint:errcheck

		// Return true for 2XX status codes (healthy), false otherwise (unhealthy)
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return true, resp.Status, nil
		}

		lastStatus = resp.Status

		// If this is not the last attempt and status is not healthy, retry
		if attempt < maxRetries {
			time.Sleep(retryDelay)
			continue
		}

		return false, lastStatus, nil
	}

	// This should never be reached, but return the last known state
	return false, lastStatus, nil
}
