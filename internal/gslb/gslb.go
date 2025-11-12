package gslb

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"
)

type Gslb interface {
	CheckPrimaryHealth() (bool, error)
	PrimaryIP() string
	SecondaryIP() string
	GetCurrentIP() (string, error)
	SwitchToPrimaryIP() error
	SwitchToSecondaryIP() error
}

type HealthChecker interface {
	CheckHealth() (bool, string, error)
}

type GslbConfig struct {
	Host                 string
	PrimaryIP            string
	SecondaryIP          string
	PrimaryHealthChecker HealthChecker
}

func eval(o Gslb) error {
	// Check primary health
	healthy, err := o.CheckPrimaryHealth()
	if err != nil {
		return fmt.Errorf("checking primary health: %w", err)
	}

	// Get GSLB record state
	rec, err := o.GetCurrentIP()
	if err != nil {
		return fmt.Errorf("getting GSLB record IP: %w", err)
	}

	if healthy && !compareIPs(rec, o.PrimaryIP()) {
		// Switch to primary IP if primary is healthy
		if err := o.SwitchToPrimaryIP(); err != nil {
			return fmt.Errorf("updating GSLB record to primary IP: %w", err)
		}

		log.Println("Switched GSLB record to primary IP:", o.PrimaryIP())
	} else if !healthy && !compareIPs(rec, o.SecondaryIP()) {
		// Switch to secondary IP if primary is not healthy
		if err := o.SwitchToSecondaryIP(); err != nil {
			return fmt.Errorf("updating GSLB record to secondary IP: %w", err)
		}

		log.Println("Switched GSLB record to secondary IP:", o.SecondaryIP())
	}

	return nil
}

func compareIPs(ip1, ip2 string) bool {
	// Parse IP addresses
	addr1 := net.ParseIP(ip1)
	addr2 := net.ParseIP(ip2)

	if addr1 == nil || addr2 == nil {
		return false
	}

	return addr1.Equal(addr2)
}

func Run(ctx context.Context, o Gslb, interval time.Duration) error {
	for {
		select {
		case <-time.After(interval):
			// Perform the health check and update the GSLB record
			if err := eval(o); err != nil {
				log.Println("Error during GSLB evaluation:", err)
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
