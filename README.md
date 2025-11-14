# GSLB Switcher

A Global Server Load Balancing (GSLB) switcher that monitors the health of a primary server and automatically switches DNS records between primary and secondary IP addresses based on health check results.

## Overview

This tool provides automated failover for DNS-based load balancing by continuously monitoring a primary server's health and updating DNS records in OpnSense Unbound DNS when the primary becomes unavailable or recovers. It ensures high availability by seamlessly switching traffic to a secondary server or load balancer when the primary fails health checks.

### Features

- **Health Monitoring**: Continuously checks the primary server's availability
- **Failover**: Automatically switches to secondary when primary is unhealthy, and back to primary when it recovers
- **OpnSense Integration**: Works with OpnSense Unbound DNS Host Override records
- **HTTP Health Checks**: Monitors HTTP/HTTPS endpoints for availability

Currently supported DNS providers:

- OpnSense Unbound DNS (Host Override records)

Currently supported health checks:

- Basic HTTP/HTTPS endpoint check (status code 200-299 = healthy)
  - Includes automatic retry mechanism: 3 attempts with 10-second delays between retries
  - 5-second timeout per request attempt

## How It Works

### Switching Logic

The GSLB switcher follows a simple yet effective decision-making process:

1. **Health Check**: Every 60 seconds, the tool performs an HTTP GET request to the configured primary health check URL
   
2. **Evaluate Health Status**:
   - **Healthy**: HTTP status code 200-299
   - **Unhealthy**: Any other status code, timeout, or connection error (after all retries are exhausted)

3. **Decision Making**:
   - If **primary is healthy** AND **DNS points to secondary** → Switch to primary IP
   - If **primary is unhealthy** AND **DNS points to primary** → Switch to secondary IP
   - If **DNS already points to the correct IP** → No action taken

This logic ensures that:
- Traffic always flows to the healthy server
- Unnecessary DNS updates are avoided
- Primary server is preferred when healthy (automatic failback)

## Configuration

The application is configured entirely through environment variables:

### Required Environment Variables

| Variable | Description | Example |
|----------|-------------|---------|
| `GSLB_HOST` | The hostname/FQDN managed by GSLB | `api.example.com` |
| `GSLB_PRIMARY_IP` | IP address of the primary server | `10.0.0.101` |
| `GSLB_PRIMARY_CHECK` | HTTP(S) URL to check primary health | `https://10.0.0.101:443/health` |
| `GSLB_SECONDARY_IP` | IP address of the secondary/failover server | `10.0.0.102` |
| `OPNSENSE_HOST` | OpnSense API endpoint base URL | `https://firewall.example.com` |
| `OPNSENSE_AUTH` | OpnSense API authentication credentials | `key:secret` |

### Optional Environment Variables

| Variable | Description | Default | Example |
|----------|-------------|---------|---------|
| `GSLB_PRIMARY_CHECK_SKIP_TLS_VERIFY` | Skip TLS certificate verification for health checks | `false` | `true` |

Configuration Example:

```bash
export GSLB_HOST="k8s-apiserver.local"
export GSLB_PRIMARY_IP="10.0.0.201"
export GSLB_PRIMARY_CHECK="https://10.0.0.201:6443/healthz"
export GSLB_SECONDARY_IP="10.0.0.202"
export OPNSENSE_HOST="https://opnsense.local"
export OPNSENSE_AUTH="your-api-key:your-api-secret"

# Optional: Skip TLS verification for health checks
# export GSLB_PRIMARY_CHECK_SKIP_TLS_VERIFY="true"
```

### Docker Compose Example

```yaml
version: '3.8'

services:
  gslb-switcher:
    image: ghcr.io/microfast-ch/gslb-switcher:latest
    environment:
      - GSLB_HOST=api.example.com
      - GSLB_PRIMARY_IP=10.0.0.101
      - GSLB_PRIMARY_CHECK=https://10.0.0.101:443/health
      - GSLB_SECONDARY_IP=10.0.0.102
      - OPNSENSE_HOST=https://firewall.example.com
      - OPNSENSE_AUTH=key:secret
      # Optional: Skip TLS verification for health checks
      # - GSLB_PRIMARY_CHECK_SKIP_TLS_VERIFY=true
    restart: unless-stopped
```

## Prerequisites

### OpnSense Setup

1. **Enable Unbound DNS** in OpnSense
2. **Create a Host Override** for your managed hostname:
   - Navigate to Services → Unbound DNS → Overrides
   - Add Host Override with your hostname and initial IP address
   - Must be an A (IPv4) or AAAA (IPv6) record
3. **Generate API Credentials**:
   - Navigate to System → Access → Users
   - Create or edit a user
   - Generate API key and secret
   - Ensure the user has permissions to access Unbound API endpoints

### Health Check Endpoint

Your primary server should expose an HTTP(S) endpoint that returns:
- Status code 200-299 when healthy
- Any other status code, timeout, or connection refused when unhealthy
