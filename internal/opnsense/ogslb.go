package opnsense

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/microfast-ch/gslb-switcher/internal/gslb"
)

type OpnSenseGslb struct {
	cfg        gslb.GslbConfig
	recordUUID string
	epHost     string
	epAuth     string
}

func NewOpnSenseGslb(host, auth string, cfg gslb.GslbConfig) (gslb.Gslb, error) {
	o := &OpnSenseGslb{
		cfg:    cfg,
		epHost: host,
		epAuth: auth,
	}

	uuid, err := o.getGslbRecordUUID(cfg.Host)
	if err != nil {
		return nil, fmt.Errorf("getting GSLB record: %w", err)
	}

	o.recordUUID = uuid

	return o, nil
}

// setAuthHeader adds the Basic Authentication header to an HTTP request
func (o *OpnSenseGslb) setAuthHeader(req *http.Request) {
	if o.epAuth != "" {
		encodedAuth := base64.StdEncoding.EncodeToString([]byte(o.epAuth))
		req.Header.Set("Authorization", "Basic "+encodedAuth)
	}
}

func (o *OpnSenseGslb) doRequest(method, url string, body []byte) (*http.Response, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest(method, o.epHost+url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	o.setAuthHeader(req)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}

	return resp, nil
}

// PrimaryIP implements gslb.Gslb.
func (o *OpnSenseGslb) PrimaryIP() string {
	return o.cfg.PrimaryIP
}

// SecondaryIP implements gslb.Gslb.
func (o *OpnSenseGslb) SecondaryIP() string {
	return o.cfg.SecondaryIP
}

type unboundSearchHostOverrideRequest struct {
	RowCount     int    `json:"rowCount"`
	SearchPhrase string `json:"searchPhrase"`
}

type unboundSearchHostOverrideResponse struct {
	Rows []struct {
		UUID           string `json:"uuid"`
		Hostname       string `json:"hostname"`
		Domain         string `json:"domain"`
		ResourceRecord string `json:"rr"`
	} `json:"rows"`
}

func (o *OpnSenseGslb) getGslbRecordUUID(hostname string) (string, error) {
	// Prepare searchHostOverride request payload
	reqPayload := &unboundSearchHostOverrideRequest{
		RowCount:     10,
		SearchPhrase: hostname,
	}

	payload, err := json.Marshal(reqPayload)
	if err != nil {
		return "", fmt.Errorf("marshaling searchHostOverride request payload: %w", err)
	}

	// Search Host Override records
	resp, err := o.doRequest(http.MethodPost, "/api/unbound/settings/searchHostOverride/", payload)
	if err != nil {
		return "", fmt.Errorf("searchHostOverride request: %w", err)
	}

	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("searchHostOverride request failed: %s", resp.Status)
	}

	// Decode response
	var searchResp unboundSearchHostOverrideResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return "", fmt.Errorf("decoding searchHostOverride response: %w", err)
	}

	// Find record by hostname
	uuid := ""
	for _, row := range searchResp.Rows {
		if !strings.HasPrefix(row.ResourceRecord, "A ") && !strings.HasPrefix(row.ResourceRecord, "AAAA ") {
			// We only care about A and AAAA records
			continue
		}

		if row.Hostname == hostname || row.Hostname+"."+row.Domain == hostname {
			// We found the record, check if it's the first one we found
			if uuid != "" {
				return "", fmt.Errorf("multiple GSLB records found for hostname %s", hostname)
			}

			uuid = row.UUID
		}
	}

	if uuid == "" {
		return "", fmt.Errorf("no GSLB record found for hostname %s", hostname)
	}

	return uuid, nil
}

// CheckPrimaryHealth implements gslb.Gslb.
func (o *OpnSenseGslb) CheckPrimaryHealth() (bool, error) {
	// No special logic needed, pass directly to endpoint checker
	ok, status, err := o.cfg.PrimaryHealthChecker.CheckHealth()
	if err != nil {
		return false, fmt.Errorf("checking primary health: %w", err)
	}

	if !ok {
		fmt.Printf("Primary health check failed: %s\n", status)
	}

	return ok, nil
}

type unboundGetHostOverrideResponse struct {
	Host struct {
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
	} `json:"host"`
}

// GetCurrentIP implements gslb.Gslb.
func (o *OpnSenseGslb) getHostOverride() (*unboundGetHostOverrideResponse, error) {
	// Get Host Override record
	resp, err := o.doRequest(http.MethodGet, "/api/unbound/settings/getHostOverride/"+o.recordUUID, nil)
	if err != nil {
		return nil, fmt.Errorf("getHostOverride request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("getHostOverride request failed: %s", resp.Status)
	}

	// Decode response
	var getResp unboundGetHostOverrideResponse
	if err := json.NewDecoder(resp.Body).Decode(&getResp); err != nil {
		return nil, fmt.Errorf("decoding getHostOverride response: %w", err)
	}

	return &getResp, nil
}

func (o *OpnSenseGslb) GetCurrentIP() (string, error) {
	override, err := o.getHostOverride()
	if err != nil {
		return "", fmt.Errorf("getting host override: %w", err)
	}

	switch {
	case override.Host.RR.A.Selected == 1 && override.Host.Server == "":
		return "", fmt.Errorf("host override record has A record selected but no server IP set")
	case override.Host.RR.AAAA.Selected == 1 && override.Host.Server == "":
		return "", fmt.Errorf("host override record has AAAA record selected but no server IP set")
	case override.Host.RR.A.Selected == 1:
		return override.Host.Server, nil
	case override.Host.RR.AAAA.Selected == 1:
		return override.Host.Server, nil
	default:
		return "", fmt.Errorf("host override record has no A or AAAA record selected")
	}
}

type unboundSetHostOverrideRequest struct {
	Host struct {
		Enabled     string `json:"enabled"`
		Hostname    string `json:"hostname"`
		Domain      string `json:"domain"`
		RR          string `json:"rr"`
		MXPrio      string `json:"mxprio"`
		MX          string `json:"mx"`
		TTL         string `json:"ttl"`
		Server      string `json:"server"`
		Description string `json:"description"`
	} `json:"host"`
}

type unboundSetHostOverrideResponse struct {
	Result string `json:"result"`
}

func (o *OpnSenseGslb) switchToIP(ip string) error {
	override, err := o.getHostOverride()
	if err != nil {
		return fmt.Errorf("getting host override: %w", err)
	}

	var rr string
	switch {
	case override.Host.RR.A.Selected == 1:
		rr = "A"
	case override.Host.RR.AAAA.Selected == 1:
		rr = "AAAA"
	default:
		return fmt.Errorf("host override record has no A or AAAA record selected")
	}

	// Prepare setHostOverride request payload
	reqPayload := &unboundSetHostOverrideRequest{}
	reqPayload.Host.Enabled = override.Host.Enabled
	reqPayload.Host.Hostname = override.Host.Hostname
	reqPayload.Host.Domain = override.Host.Domain
	reqPayload.Host.RR = rr
	reqPayload.Host.MXPrio = override.Host.MXPrio
	reqPayload.Host.MX = override.Host.MX
	reqPayload.Host.TTL = override.Host.TTL
	reqPayload.Host.Server = ip
	reqPayload.Host.Description = override.Host.Description

	payload, err := json.Marshal(reqPayload)
	if err != nil {
		return fmt.Errorf("marshaling setHostOverride request payload: %w", err)
	}

	// Send setHostOverride request
	resp, err := o.doRequest(http.MethodPost, "/api/unbound/settings/setHostOverride/"+o.recordUUID, payload)
	if err != nil {
		return fmt.Errorf("setHostOverride request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("setHostOverride request failed: %s", resp.Status)
	}

	// Decode response
	var setResp unboundSetHostOverrideResponse
	if err := json.NewDecoder(resp.Body).Decode(&setResp); err != nil {
		return fmt.Errorf("decoding setHostOverride response: %w", err)
	}

	if setResp.Result != "saved" {
		return fmt.Errorf("setHostOverride request failed: unexpected result %s", setResp.Result)
	}

	// Restart Unbound service to apply changes
	err = o.restartUnboundService()
	if err != nil {
		return fmt.Errorf("restarting Unbound service: %w", err)
	}

	return nil
}

type unboundServiceResponse struct {
	Response string `json:"response"`
}

func (o *OpnSenseGslb) restartUnboundService() error {
	resp, err := o.doRequest(http.MethodPost, "/api/unbound/service/reconfigure", nil)
	if err != nil {
		return fmt.Errorf("reconfigure request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("reconfigure request failed: %s", resp.Status)
	}

	// Decode response
	var setResp unboundServiceResponse
	if err := json.NewDecoder(resp.Body).Decode(&setResp); err != nil {
		return fmt.Errorf("decoding reconfigure response: %w", err)
	}

	if setResp.Response != "OK" {
		return fmt.Errorf("reconfigure request failed: unexpected result %s", setResp.Response)
	}

	return nil
}

// SwitchToPrimaryIP implements gslb.Gslb.
func (o *OpnSenseGslb) SwitchToPrimaryIP() error {
	return o.switchToIP(o.PrimaryIP())
}

// SwitchToSecondaryIP implements gslb.Gslb.
func (o *OpnSenseGslb) SwitchToSecondaryIP() error {
	return o.switchToIP(o.SecondaryIP())
}
