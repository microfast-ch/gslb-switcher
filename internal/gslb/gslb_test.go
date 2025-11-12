package gslb

import "testing"

type mockGslb struct {
	primaryIPval   string
	secondaryIPval string
	IsPrimaryUp    bool
	currentIP      string
}

func newMockGslb() *mockGslb {
	return &mockGslb{
		primaryIPval:   "10.0.1.1",
		secondaryIPval: "20.0.2.2",
		IsPrimaryUp:    true,
		currentIP:      "10.0.1.1",
	}
}

func (m *mockGslb) CheckPrimaryHealth() (bool, error) {
	return m.IsPrimaryUp, nil
}

func (m *mockGslb) GetCurrentIP() (string, error) {
	return m.currentIP, nil
}

func (m *mockGslb) PrimaryIP() string {
	return m.primaryIPval
}

func (m *mockGslb) SecondaryIP() string {
	return m.secondaryIPval
}

func (m *mockGslb) SwitchToPrimaryIP() error {
	m.currentIP = m.PrimaryIP()
	return nil
}

func (m *mockGslb) SwitchToSecondaryIP() error {
	m.currentIP = m.SecondaryIP()
	return nil
}

func TestGslbEval(t *testing.T) {
	// Create mock GSLB
	g := newMockGslb()
	var _ Gslb = g // Ensure mockGslb implements Gslb interface
	if err := eval(g); err != nil {
		t.Fatalf("eval() failed: %v", err)
	}

	// Check initial state
	curIP, err := g.GetCurrentIP()
	if err != nil {
		t.Fatalf("GetCurrentIP() failed: %v", err)
	}
	if curIP != g.PrimaryIP() {
		t.Fatalf("expected CurrentIP to be PrimaryIP (%s), got %s", g.PrimaryIP(), curIP)
	}

	// Simulate primary down
	g.IsPrimaryUp = false
	if err := eval(g); err != nil {
		t.Fatalf("eval() failed: %v", err)
	}

	curIP, err = g.GetCurrentIP()
	if err != nil {
		t.Fatalf("GetCurrentIP() failed: %v", err)
	}
	if curIP != g.SecondaryIP() {
		t.Fatalf("expected CurrentIP to be SecondaryIP (%s), got %s", g.SecondaryIP(), curIP)
	}

	// Simulate primary up again
	g.IsPrimaryUp = true
	if err := eval(g); err != nil {
		t.Fatalf("eval() failed: %v", err)
	}

	curIP, err = g.GetCurrentIP()
	if err != nil {
		t.Fatalf("GetCurrentIP() failed: %v", err)
	}
	if curIP != g.PrimaryIP() {
		t.Fatalf("expected CurrentIP to be PrimaryIP (%s), got %s", g.PrimaryIP(), curIP)
	}
}
