package opnsense

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/microfast-ch/gslb-switcher/internal/gslb"
)

func TestNewOpnSenseGslb_Success(t *testing.T) {
	// Create a test server that returns a valid GSLB record
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/unbound/settings/searchHostOverride/" {
			response := unboundSearchHostOverrideResponse{
				Rows: []struct {
					UUID           string `json:"uuid"`
					Hostname       string `json:"hostname"`
					Domain         string `json:"domain"`
					ResourceRecord string `json:"rr"`
				}{
					{
						UUID:           "test-uuid-123",
						Hostname:       "test-host",
						Domain:         "local",
						ResourceRecord: "A (IPv4 address)",
					},
				},
			}
			json.NewEncoder(w).Encode(response) // nolint:errcheck
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	cfg := gslb.GslbConfig{
		Host:        "test-host",
		PrimaryIP:   "10.0.0.1",
		SecondaryIP: "10.0.0.2",
	}

	gslbImpl, err := NewOpnSenseGslb(server.URL, "test-auth", cfg)
	if err != nil {
		t.Fatalf("NewOpnSenseGslb failed: %v", err)
	}

	if gslbImpl == nil {
		t.Fatal("Expected non-nil GSLB implementation")
	}

	opnsense := gslbImpl.(*OpnSenseGslb)
	if opnsense.recordUUID != "test-uuid-123" {
		t.Errorf("Expected recordUUID to be 'test-uuid-123', got '%s'", opnsense.recordUUID)
	}
}

func TestNewOpnSenseGslb_MultipleRecordsError(t *testing.T) {
	// Create a mock server that returns multiple matching records
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := unboundSearchHostOverrideResponse{
			Rows: []struct {
				UUID           string `json:"uuid"`
				Hostname       string `json:"hostname"`
				Domain         string `json:"domain"`
				ResourceRecord string `json:"rr"`
			}{
				{
					UUID:           "uuid-1",
					Hostname:       "test-host",
					Domain:         "local",
					ResourceRecord: "A (IPv4 address)",
				},
				{
					UUID:           "uuid-2",
					Hostname:       "test-host",
					Domain:         "local",
					ResourceRecord: "A (IPv4 address)",
				},
			},
		}
		json.NewEncoder(w).Encode(resp) // nolint:errcheck
	}))
	defer server.Close()

	cfg := gslb.GslbConfig{
		Host:        "test-host",
		PrimaryIP:   "10.0.0.1",
		SecondaryIP: "10.0.0.2",
	}

	_, err := NewOpnSenseGslb(server.URL, "", cfg)
	if err == nil {
		t.Fatal("Expected error for multiple records, got nil")
	}

	if !strings.Contains(err.Error(), "multiple GSLB records found") {
		t.Errorf("Expected 'multiple GSLB records found' error, got: %v", err)
	}
}

func TestNewOpnSenseGslb_NoRecordsError(t *testing.T) {
	// Create a mock server that returns no matching records
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := unboundSearchHostOverrideResponse{
			Rows: []struct {
				UUID           string `json:"uuid"`
				Hostname       string `json:"hostname"`
				Domain         string `json:"domain"`
				ResourceRecord string `json:"rr"`
			}{},
		}
		json.NewEncoder(w).Encode(resp) // nolint:errcheck
	}))
	defer server.Close()

	cfg := gslb.GslbConfig{
		Host:        "test-host",
		PrimaryIP:   "10.0.0.1",
		SecondaryIP: "10.0.0.2",
	}

	_, err := NewOpnSenseGslb(server.URL, "", cfg)
	if err == nil {
		t.Fatal("Expected error for no records, got nil")
	}

	if !strings.Contains(err.Error(), "no GSLB record found") {
		t.Errorf("Expected 'no GSLB record found' error, got: %v", err)
	}
}

func TestNewOpnSenseGslb_IgnoresNonARecords(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := unboundSearchHostOverrideResponse{
			Rows: []struct {
				UUID           string `json:"uuid"`
				Hostname       string `json:"hostname"`
				Domain         string `json:"domain"`
				ResourceRecord string `json:"rr"`
			}{
				{
					UUID:           "mx-record",
					Hostname:       "test-host",
					Domain:         "local",
					ResourceRecord: "MX (Mail server)",
				},
				{
					UUID:           "a-record",
					Hostname:       "test-host",
					Domain:         "local",
					ResourceRecord: "A (IPv4 address)",
				},
			},
		}
		json.NewEncoder(w).Encode(resp) // nolint:errcheck
	}))
	defer server.Close()

	cfg := gslb.GslbConfig{
		Host:        "test-host",
		PrimaryIP:   "10.0.0.1",
		SecondaryIP: "10.0.0.2",
	}

	g, err := NewOpnSenseGslb(server.URL, "", cfg)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	opnGslb, ok := g.(*OpnSenseGslb)
	if !ok {
		t.Fatal("Expected OpnSenseGslb type")
	}

	if opnGslb.recordUUID != "a-record" {
		t.Errorf("Expected recordUUID to be 'a-record', got '%s'", opnGslb.recordUUID)
	}
}

func TestNewOpnSenseGslb_MatchesHostnameDomain(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return a record where hostname.domain matches the search
		resp := unboundSearchHostOverrideResponse{
			Rows: []struct {
				UUID           string `json:"uuid"`
				Hostname       string `json:"hostname"`
				Domain         string `json:"domain"`
				ResourceRecord string `json:"rr"`
			}{
				{
					UUID:           "correct-uuid",
					Hostname:       "test",
					Domain:         "example.com",
					ResourceRecord: "A (IPv4 address)",
				},
			},
		}
		json.NewEncoder(w).Encode(resp) // nolint:errcheck
	}))
	defer server.Close()

	cfg := gslb.GslbConfig{
		Host:        "test.example.com",
		PrimaryIP:   "10.0.0.1",
		SecondaryIP: "10.0.0.2",
	}

	g, err := NewOpnSenseGslb(server.URL, "", cfg)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	opnGslb, ok := g.(*OpnSenseGslb)
	if !ok {
		t.Fatal("Expected OpnSenseGslb type")
	}

	if opnGslb.recordUUID != "correct-uuid" {
		t.Errorf("Expected recordUUID to be 'correct-uuid', got '%s'", opnGslb.recordUUID)
	}
}

func TestIPRecords(t *testing.T) {
	o := &OpnSenseGslb{
		cfg: gslb.GslbConfig{
			PrimaryIP:   "192.168.1.1",
			SecondaryIP: "192.168.1.2",
		},
	}

	if o.PrimaryIP() != "192.168.1.1" {
		t.Errorf("Expected PrimaryIP to be '192.168.1.1', got '%s'", o.PrimaryIP())
	}

	if o.SecondaryIP() != "192.168.1.2" {
		t.Errorf("Expected SecondaryIP to be '192.168.1.2', got '%s'", o.SecondaryIP())
	}
}

func TestGetCurrentIP_ARecord(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("Expected GET request, got %s", r.Method)
		}
		if !strings.Contains(r.URL.Path, "/api/unbound/settings/getHostOverride/") {
			t.Errorf("Unexpected URL path: %s", r.URL.Path)
		}

		resp := unboundGetHostOverrideResponse{
			Host: struct {
				Enabled  string `json:"enabled"`
				Hostname string `json:"hostname"`
				Domain   string `json:"domain"`
				RR       struct {
					A struct {
						Value    string `json:"value"`
						Selected int    `json:"selected"`
					} `json:"A"`
					AAAA struct {
						Value    string `json:"value"`
						Selected int    `json:"selected"`
					} `json:"AAAA"`
				} `json:"rr"`
				MXPrio      string `json:"mxprio"`
				MX          string `json:"mx"`
				TTL         string `json:"ttl"`
				Server      string `json:"server"`
				Description string `json:"description"`
			}{
				Enabled:  "1",
				Hostname: "test",
				Domain:   "local",
				Server:   "10.0.0.1",
			},
		}
		resp.Host.RR.A.Selected = 1
		resp.Host.RR.A.Value = "A (IPv4 address)"
		json.NewEncoder(w).Encode(resp) // nolint:errcheck
	}))
	defer server.Close()

	o := &OpnSenseGslb{
		epHost:     server.URL,
		recordUUID: "test-uuid",
	}

	ip, err := o.GetCurrentIP()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if ip != "10.0.0.1" {
		t.Errorf("Expected IP to be '10.0.0.1', got '%s'", ip)
	}
}

func TestGetCurrentIP_AAAARecord(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := unboundGetHostOverrideResponse{
			Host: struct {
				Enabled  string `json:"enabled"`
				Hostname string `json:"hostname"`
				Domain   string `json:"domain"`
				RR       struct {
					A struct {
						Value    string `json:"value"`
						Selected int    `json:"selected"`
					} `json:"A"`
					AAAA struct {
						Value    string `json:"value"`
						Selected int    `json:"selected"`
					} `json:"AAAA"`
				} `json:"rr"`
				MXPrio      string `json:"mxprio"`
				MX          string `json:"mx"`
				TTL         string `json:"ttl"`
				Server      string `json:"server"`
				Description string `json:"description"`
			}{
				Enabled:  "1",
				Hostname: "test",
				Domain:   "local",
				Server:   "2001:db8::1",
			},
		}
		resp.Host.RR.AAAA.Selected = 1
		resp.Host.RR.AAAA.Value = "AAAA (IPv6 address)"
		json.NewEncoder(w).Encode(resp) // nolint:errcheck
	}))
	defer server.Close()

	o := &OpnSenseGslb{
		epHost:     server.URL,
		recordUUID: "test-uuid",
	}

	ip, err := o.GetCurrentIP()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if ip != "2001:db8::1" {
		t.Errorf("Expected IP to be '2001:db8::1', got '%s'", ip)
	}
}

func TestGetCurrentIP_NoRecordSelected(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := unboundGetHostOverrideResponse{
			Host: struct {
				Enabled  string `json:"enabled"`
				Hostname string `json:"hostname"`
				Domain   string `json:"domain"`
				RR       struct {
					A struct {
						Value    string `json:"value"`
						Selected int    `json:"selected"`
					} `json:"A"`
					AAAA struct {
						Value    string `json:"value"`
						Selected int    `json:"selected"`
					} `json:"AAAA"`
				} `json:"rr"`
				MXPrio      string `json:"mxprio"`
				MX          string `json:"mx"`
				TTL         string `json:"ttl"`
				Server      string `json:"server"`
				Description string `json:"description"`
			}{
				Enabled:  "1",
				Hostname: "test",
				Domain:   "local",
				Server:   "10.0.0.1",
			},
		}
		// Neither A nor AAAA is selected
		resp.Host.RR.A.Selected = 0
		resp.Host.RR.AAAA.Selected = 0
		json.NewEncoder(w).Encode(resp) // nolint:errcheck
	}))
	defer server.Close()

	o := &OpnSenseGslb{
		epHost:     server.URL,
		recordUUID: "test-uuid",
	}

	_, err := o.GetCurrentIP()
	if err == nil {
		t.Fatal("Expected error for no record selected, got nil")
	}

	if !strings.Contains(err.Error(), "no A or AAAA record selected") {
		t.Errorf("Expected 'no A or AAAA record selected' error, got: %v", err)
	}
}

func TestGetCurrentIP_EmptyServerWithARecord(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := unboundGetHostOverrideResponse{
			Host: struct {
				Enabled  string `json:"enabled"`
				Hostname string `json:"hostname"`
				Domain   string `json:"domain"`
				RR       struct {
					A struct {
						Value    string `json:"value"`
						Selected int    `json:"selected"`
					} `json:"A"`
					AAAA struct {
						Value    string `json:"value"`
						Selected int    `json:"selected"`
					} `json:"AAAA"`
				} `json:"rr"`
				MXPrio      string `json:"mxprio"`
				MX          string `json:"mx"`
				TTL         string `json:"ttl"`
				Server      string `json:"server"`
				Description string `json:"description"`
			}{
				Enabled:  "1",
				Hostname: "test",
				Domain:   "local",
				Server:   "", // Empty server IP
			},
		}
		resp.Host.RR.A.Selected = 1
		json.NewEncoder(w).Encode(resp) // nolint:errcheck
	}))
	defer server.Close()

	o := &OpnSenseGslb{
		epHost:     server.URL,
		recordUUID: "test-uuid",
	}

	_, err := o.GetCurrentIP()
	if err == nil {
		t.Fatal("Expected error for empty server IP, got nil")
	}

	if !strings.Contains(err.Error(), "no server IP set") {
		t.Errorf("Expected 'no server IP set' error, got: %v", err)
	}
}

func TestSwitchToPrimaryIP(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++

		switch callCount {
		case 1:
			// First call: getHostOverride
			if r.Method != "GET" {
				t.Errorf("Expected GET request for getHostOverride, got %s", r.Method)
			}

			resp := unboundGetHostOverrideResponse{
				Host: struct {
					Enabled  string `json:"enabled"`
					Hostname string `json:"hostname"`
					Domain   string `json:"domain"`
					RR       struct {
						A struct {
							Value    string `json:"value"`
							Selected int    `json:"selected"`
						} `json:"A"`
						AAAA struct {
							Value    string `json:"value"`
							Selected int    `json:"selected"`
						} `json:"AAAA"`
					} `json:"rr"`
					MXPrio      string `json:"mxprio"`
					MX          string `json:"mx"`
					TTL         string `json:"ttl"`
					Server      string `json:"server"`
					Description string `json:"description"`
				}{
					Enabled:     "1",
					Hostname:    "test",
					Domain:      "local",
					Server:      "10.0.0.2",
					TTL:         "300",
					Description: "Test record",
				},
			}
			resp.Host.RR.A.Selected = 1
			json.NewEncoder(w).Encode(resp) // nolint:errcheck
		case 2:
			// Second call: setHostOverride
			if r.Method != "POST" {
				t.Errorf("Expected POST request for setHostOverride, got %s", r.Method)
			}
			if !strings.Contains(r.URL.Path, "/api/unbound/settings/setHostOverride/") {
				t.Errorf("Unexpected URL path: %s", r.URL.Path)
			}

			var req unboundSetHostOverrideRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("Failed to decode request: %v", err)
			}

			if req.Host.Server != "10.0.0.1" {
				t.Errorf("Expected server to be '10.0.0.1', got '%s'", req.Host.Server)
			}

			resp := unboundSetHostOverrideResponse{
				Result: "saved",
			}
			json.NewEncoder(w).Encode(resp) // nolint:errcheck
		case 3:
			// Third call: reconfigure service
			if r.Method != "POST" {
				t.Errorf("Expected POST request for reconfigure, got %s", r.Method)
			}
			if !strings.Contains(r.URL.Path, "/api/unbound/service/reconfigure") {
				t.Errorf("Unexpected URL path: %s", r.URL.Path)
			}

			resp := unboundServiceResponse{
				Response: "OK",
			}
			json.NewEncoder(w).Encode(resp) // nolint:errcheck
		}
	}))
	defer server.Close()

	o := &OpnSenseGslb{
		cfg: gslb.GslbConfig{
			PrimaryIP: "10.0.0.1",
		},
		epHost:     server.URL,
		recordUUID: "test-uuid",
	}

	err := o.SwitchToPrimaryIP()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if callCount != 3 {
		t.Errorf("Expected 3 API calls, got %d", callCount)
	}
}

func TestSwitchToSecondaryIP(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++

		switch callCount {
		case 1:
			// First call: getHostOverride
			resp := unboundGetHostOverrideResponse{
				Host: struct {
					Enabled  string `json:"enabled"`
					Hostname string `json:"hostname"`
					Domain   string `json:"domain"`
					RR       struct {
						A struct {
							Value    string `json:"value"`
							Selected int    `json:"selected"`
						} `json:"A"`
						AAAA struct {
							Value    string `json:"value"`
							Selected int    `json:"selected"`
						} `json:"AAAA"`
					} `json:"rr"`
					MXPrio      string `json:"mxprio"`
					MX          string `json:"mx"`
					TTL         string `json:"ttl"`
					Server      string `json:"server"`
					Description string `json:"description"`
				}{
					Enabled:     "1",
					Hostname:    "test",
					Domain:      "local",
					Server:      "10.0.0.1",
					TTL:         "300",
					Description: "Test record",
				},
			}
			resp.Host.RR.A.Selected = 1
			json.NewEncoder(w).Encode(resp) // nolint:errcheck
		case 2:
			// Second call: setHostOverride
			var req unboundSetHostOverrideRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("Failed to decode request: %v", err)
			}

			if req.Host.Server != "10.0.0.2" {
				t.Errorf("Expected server to be '10.0.0.2', got '%s'", req.Host.Server)
			}

			resp := unboundSetHostOverrideResponse{
				Result: "saved",
			}
			json.NewEncoder(w).Encode(resp) // nolint:errcheck
		case 3:
			// Third call: reconfigure service
			resp := unboundServiceResponse{
				Response: "OK",
			}
			json.NewEncoder(w).Encode(resp) // nolint:errcheck
		}
	}))
	defer server.Close()

	o := &OpnSenseGslb{
		cfg: gslb.GslbConfig{
			SecondaryIP: "10.0.0.2",
		},
		epHost:     server.URL,
		recordUUID: "test-uuid",
	}

	err := o.SwitchToSecondaryIP()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if callCount != 3 {
		t.Errorf("Expected 3 API calls, got %d", callCount)
	}
}

func TestSwitchToIP_SetHostOverrideFailure(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++

		switch callCount {
		case 1:
			// First call: getHostOverride
			resp := unboundGetHostOverrideResponse{
				Host: struct {
					Enabled  string `json:"enabled"`
					Hostname string `json:"hostname"`
					Domain   string `json:"domain"`
					RR       struct {
						A struct {
							Value    string `json:"value"`
							Selected int    `json:"selected"`
						} `json:"A"`
						AAAA struct {
							Value    string `json:"value"`
							Selected int    `json:"selected"`
						} `json:"AAAA"`
					} `json:"rr"`
					MXPrio      string `json:"mxprio"`
					MX          string `json:"mx"`
					TTL         string `json:"ttl"`
					Server      string `json:"server"`
					Description string `json:"description"`
				}{
					Enabled:  "1",
					Hostname: "test",
					Domain:   "local",
					Server:   "10.0.0.2",
				},
			}
			resp.Host.RR.A.Selected = 1
			json.NewEncoder(w).Encode(resp) // nolint:errcheck
		case 2:
			// Second call: setHostOverride returns error
			resp := unboundSetHostOverrideResponse{
				Result: "failed",
			}
			json.NewEncoder(w).Encode(resp) // nolint:errcheck
		}
	}))
	defer server.Close()

	o := &OpnSenseGslb{
		cfg: gslb.GslbConfig{
			PrimaryIP: "10.0.0.1",
		},
		epHost:     server.URL,
		recordUUID: "test-uuid",
	}

	err := o.SwitchToPrimaryIP()
	if err == nil {
		t.Fatal("Expected error for setHostOverride failure, got nil")
	}

	if !strings.Contains(err.Error(), "unexpected result") {
		t.Errorf("Expected 'unexpected result' error, got: %v", err)
	}
}

func TestSwitchToIP_ReconfigureFailure(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++

		switch callCount {
		case 1:
			// First call: getHostOverride
			resp := unboundGetHostOverrideResponse{
				Host: struct {
					Enabled  string `json:"enabled"`
					Hostname string `json:"hostname"`
					Domain   string `json:"domain"`
					RR       struct {
						A struct {
							Value    string `json:"value"`
							Selected int    `json:"selected"`
						} `json:"A"`
						AAAA struct {
							Value    string `json:"value"`
							Selected int    `json:"selected"`
						} `json:"AAAA"`
					} `json:"rr"`
					MXPrio      string `json:"mxprio"`
					MX          string `json:"mx"`
					TTL         string `json:"ttl"`
					Server      string `json:"server"`
					Description string `json:"description"`
				}{
					Enabled:  "1",
					Hostname: "test",
					Domain:   "local",
					Server:   "10.0.0.2",
				},
			}
			resp.Host.RR.A.Selected = 1
			json.NewEncoder(w).Encode(resp) // nolint:errcheck
		case 2:
			// Second call: setHostOverride succeeds
			resp := unboundSetHostOverrideResponse{
				Result: "saved",
			}
			json.NewEncoder(w).Encode(resp) // nolint:errcheck
		case 3:
			// Third call: reconfigure fails
			resp := unboundServiceResponse{
				Response: "FAILED",
			}
			json.NewEncoder(w).Encode(resp) // nolint:errcheck
		}
	}))
	defer server.Close()

	o := &OpnSenseGslb{
		cfg: gslb.GslbConfig{
			PrimaryIP: "10.0.0.1",
		},
		epHost:     server.URL,
		recordUUID: "test-uuid",
	}

	err := o.SwitchToPrimaryIP()
	if err == nil {
		t.Fatal("Expected error for reconfigure failure, got nil")
	}

	if !strings.Contains(err.Error(), "unexpected result") {
		t.Errorf("Expected 'unexpected result' error, got: %v", err)
	}
}

func TestSwitchToIP_AAAARecord(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++

		switch callCount {
		case 1:
			// First call: getHostOverride with AAAA record
			resp := unboundGetHostOverrideResponse{
				Host: struct {
					Enabled  string `json:"enabled"`
					Hostname string `json:"hostname"`
					Domain   string `json:"domain"`
					RR       struct {
						A struct {
							Value    string `json:"value"`
							Selected int    `json:"selected"`
						} `json:"A"`
						AAAA struct {
							Value    string `json:"value"`
							Selected int    `json:"selected"`
						} `json:"AAAA"`
					} `json:"rr"`
					MXPrio      string `json:"mxprio"`
					MX          string `json:"mx"`
					TTL         string `json:"ttl"`
					Server      string `json:"server"`
					Description string `json:"description"`
				}{
					Enabled:     "1",
					Hostname:    "test",
					Domain:      "local",
					Server:      "2001:db8::2",
					TTL:         "300",
					Description: "Test record",
				},
			}
			resp.Host.RR.AAAA.Selected = 1
			json.NewEncoder(w).Encode(resp) // nolint:errcheck
		case 2:
			// Second call: setHostOverride
			var req unboundSetHostOverrideRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("Failed to decode request: %v", err)
			}

			if req.Host.Server != "2001:db8::1" {
				t.Errorf("Expected server to be '2001:db8::1', got '%s'", req.Host.Server)
			}

			if req.Host.RR != "AAAA" {
				t.Errorf("Expected RR to be 'AAAA', got '%s'", req.Host.RR)
			}

			resp := unboundSetHostOverrideResponse{
				Result: "saved",
			}
			json.NewEncoder(w).Encode(resp) // nolint:errcheck
		case 3:
			// Third call: reconfigure service
			resp := unboundServiceResponse{
				Response: "OK",
			}
			json.NewEncoder(w).Encode(resp) // nolint:errcheck
		}
	}))
	defer server.Close()

	o := &OpnSenseGslb{
		cfg: gslb.GslbConfig{
			PrimaryIP: "2001:db8::1",
		},
		epHost:     server.URL,
		recordUUID: "test-uuid",
	}

	err := o.SwitchToPrimaryIP()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if callCount != 3 {
		t.Errorf("Expected 3 API calls, got %d", callCount)
	}
}
