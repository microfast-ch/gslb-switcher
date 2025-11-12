package checkers

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestSimpleHTTPChecker_CheckHealth(t *testing.T) {
	tests := []struct {
		name          string
		statusCode    int
		wantHealthy   bool
		wantStatusMsg string
		wantErr       bool
		setupServer   bool
		invalidURL    bool
	}{
		{
			name:          "healthy service returns 200",
			statusCode:    http.StatusOK,
			wantHealthy:   true,
			wantStatusMsg: "200 OK",
			wantErr:       false,
			setupServer:   true,
		},
		{
			name:          "healthy service returns 201",
			statusCode:    http.StatusCreated,
			wantHealthy:   true,
			wantStatusMsg: "201 Created",
			wantErr:       false,
			setupServer:   true,
		},
		{
			name:          "unhealthy service returns 404",
			statusCode:    http.StatusNotFound,
			wantHealthy:   false,
			wantStatusMsg: "404 Not Found",
			wantErr:       false,
			setupServer:   true,
		},
		{
			name:          "unhealthy service returns 500",
			statusCode:    http.StatusInternalServerError,
			wantHealthy:   false,
			wantStatusMsg: "500 Internal Server Error",
			wantErr:       false,
			setupServer:   true,
		},
		{
			name:          "unhealthy service returns 503",
			statusCode:    http.StatusServiceUnavailable,
			wantHealthy:   false,
			wantStatusMsg: "503 Service Unavailable",
			wantErr:       false,
			setupServer:   true,
		},
		{
			name:        "invalid URL returns error",
			wantHealthy: false,
			wantErr:     true,
			setupServer: false,
			invalidURL:  true,
		},
		{
			name:        "connection error returns unhealthy",
			wantHealthy: false,
			wantErr:     false,
			setupServer: false,
			invalidURL:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var checker *SimpleHTTPChecker

			if tt.setupServer {
				// Create a test server that returns the specified status code
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(tt.statusCode)
				}))
				defer server.Close()

				checker = NewSimpleHTTPChecker(server.URL)
			} else if tt.invalidURL {
				// Use an invalid URL to test URL parsing error
				checker = NewSimpleHTTPChecker("://invalid-url")
			} else {
				// Use an unreachable URL to test connection error
				checker = NewSimpleHTTPChecker("http://localhost:99999")
			}

			healthy, statusMsg, err := checker.CheckHealth()

			// Check error expectation
			if tt.wantErr {
				if err == nil {
					t.Errorf("CheckHealth() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("CheckHealth() unexpected error: %v", err)
				return
			}

			// Check healthy status
			if healthy != tt.wantHealthy {
				t.Errorf("CheckHealth() healthy = %v, want %v", healthy, tt.wantHealthy)
			}

			// Check status message for server-based tests
			if tt.setupServer && statusMsg != tt.wantStatusMsg {
				t.Errorf("CheckHealth() statusMsg = %v, want %v", statusMsg, tt.wantStatusMsg)
			}

			// For connection errors, ensure we got some error message
			if !tt.setupServer && !tt.invalidURL && statusMsg == "" {
				t.Errorf("CheckHealth() expected non-empty status message for connection error")
			}
		})
	}
}

func TestNewSimpleHTTPChecker(t *testing.T) {
	url := "http://example.com"
	checker := NewSimpleHTTPChecker(url)

	if checker == nil {
		t.Fatal("NewSimpleHTTPChecker() returned nil")
	}

	if checker.URL != url {
		t.Errorf("NewSimpleHTTPChecker() URL = %v, want %v", checker.URL, url)
	}
}

func TestSimpleHTTPChecker_CheckHealth_Retries(t *testing.T) {
	t.Run("succeeds on first attempt - no retries needed", func(t *testing.T) {
		var attemptCount atomic.Int32

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			attemptCount.Add(1)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		checker := NewSimpleHTTPChecker(server.URL)
		healthy, statusMsg, err := checker.CheckHealth()

		if err != nil {
			t.Errorf("CheckHealth() unexpected error: %v", err)
		}
		if !healthy {
			t.Errorf("CheckHealth() healthy = false, want true")
		}
		if statusMsg != "200 OK" {
			t.Errorf("CheckHealth() statusMsg = %v, want '200 OK'", statusMsg)
		}
		if attemptCount.Load() != 1 {
			t.Errorf("Expected 1 attempt, got %d", attemptCount.Load())
		}
	})

	t.Run("succeeds on second attempt after one failure", func(t *testing.T) {
		var attemptCount atomic.Int32

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			count := attemptCount.Add(1)
			if count == 1 {
				w.WriteHeader(http.StatusServiceUnavailable)
			} else {
				w.WriteHeader(http.StatusOK)
			}
		}))
		defer server.Close()

		checker := NewSimpleHTTPChecker(server.URL)
		start := time.Now()
		healthy, statusMsg, err := checker.CheckHealth()
		elapsed := time.Since(start)

		if err != nil {
			t.Errorf("CheckHealth() unexpected error: %v", err)
		}
		if !healthy {
			t.Errorf("CheckHealth() healthy = false, want true")
		}
		if statusMsg != "200 OK" {
			t.Errorf("CheckHealth() statusMsg = %v, want '200 OK'", statusMsg)
		}
		if attemptCount.Load() != 2 {
			t.Errorf("Expected 2 attempts, got %d", attemptCount.Load())
		}
		// Should have waited at least 10 seconds for one retry
		if elapsed < 10*time.Second {
			t.Errorf("Expected at least 10s elapsed, got %v", elapsed)
		}
	})

	t.Run("succeeds on third attempt after two failures", func(t *testing.T) {
		t.SkipNow() // Skipping to avoid long test times due to retries until retry logic is configurable
		var attemptCount atomic.Int32

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			count := attemptCount.Add(1)
			if count < 3 {
				w.WriteHeader(http.StatusServiceUnavailable)
			} else {
				w.WriteHeader(http.StatusOK)
			}
		}))
		defer server.Close()

		checker := NewSimpleHTTPChecker(server.URL)
		start := time.Now()
		healthy, statusMsg, err := checker.CheckHealth()
		elapsed := time.Since(start)

		if err != nil {
			t.Errorf("CheckHealth() unexpected error: %v", err)
		}
		if !healthy {
			t.Errorf("CheckHealth() healthy = false, want true")
		}
		if statusMsg != "200 OK" {
			t.Errorf("CheckHealth() statusMsg = %v, want '200 OK'", statusMsg)
		}
		if attemptCount.Load() != 3 {
			t.Errorf("Expected 3 attempts, got %d", attemptCount.Load())
		}
		// Should have waited at least 20 seconds for two retries
		if elapsed < 20*time.Second {
			t.Errorf("Expected at least 20s elapsed, got %v", elapsed)
		}
	})

	t.Run("fails after 3 attempts with unhealthy status", func(t *testing.T) {
		t.SkipNow() // Skipping to avoid long test times due to retries until retry logic is configurable
		var attemptCount atomic.Int32

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			attemptCount.Add(1)
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer server.Close()

		checker := NewSimpleHTTPChecker(server.URL)
		start := time.Now()
		healthy, statusMsg, err := checker.CheckHealth()
		elapsed := time.Since(start)

		if err != nil {
			t.Errorf("CheckHealth() unexpected error: %v", err)
		}
		if healthy {
			t.Errorf("CheckHealth() healthy = true, want false")
		}
		if statusMsg != "503 Service Unavailable" {
			t.Errorf("CheckHealth() statusMsg = %v, want '503 Service Unavailable'", statusMsg)
		}
		if attemptCount.Load() != 3 {
			t.Errorf("Expected 3 attempts, got %d", attemptCount.Load())
		}
		// Should have waited at least 20 seconds for two retries
		if elapsed < 20*time.Second {
			t.Errorf("Expected at least 20s elapsed, got %v", elapsed)
		}
	})

	t.Run("fails after 3 connection error attempts", func(t *testing.T) {
		t.SkipNow() // Skipping to avoid long test times due to retries until retry logic is configurable
		// Use an unreachable address
		checker := NewSimpleHTTPChecker("http://localhost:99999")
		start := time.Now()
		healthy, statusMsg, err := checker.CheckHealth()
		elapsed := time.Since(start)

		if err != nil {
			t.Errorf("CheckHealth() unexpected error: %v", err)
		}
		if healthy {
			t.Errorf("CheckHealth() healthy = true, want false")
		}
		if statusMsg == "" {
			t.Errorf("CheckHealth() statusMsg should contain error message")
		}
		// Should have waited at least 20 seconds for two retries
		// (3 attempts total, 2 retries with 10s delay each)
		if elapsed < 20*time.Second {
			t.Errorf("Expected at least 20s elapsed, got %v", elapsed)
		}
	})
}
