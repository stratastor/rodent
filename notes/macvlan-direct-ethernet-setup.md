# MACVLAN Direct Ethernet Setup Guide

## Overview

This guide documents the setup for testing MACVLAN networking between a Mac and Raspberry Pi using a direct ethernet connection. This configuration allows MACVLAN to work on the ethernet interface while maintaining internet connectivity via WiFi.

## Network Architecture

### Physical Setup

- **Mac**: WiFi (wlan0) for internet + Ethernet (en0) for direct connection to RPI
- **RPI**: WiFi (wlan0) for internet + Ethernet (eth0) for direct connection to Mac
- **Cable**: Direct ethernet cable between Mac and RPI (no router/switch needed)

### IP Addressing

#### Internet Network (WiFi - 192.168.31.0/24)

- Mac wlan0: `192.168.31.173` (DHCP from main router)
- RPI wlan0: `192.168.31.116` (DHCP from main router)
- Gateway: `192.168.31.1`

#### Direct Ethernet Network (192.168.100.0/24)

- Mac ethernet (en0): `192.168.100.1/24` (static)
- RPI eth0: `192.168.100.2/24` (static via netplan)
- MACVLAN containers: `192.168.100.10+` (e.g., Samba AD DC at 192.168.100.10)

## Setup Steps

### 1. Physical Connection

Connect ethernet cable directly between Mac and RPI.

### 2. Configure Mac Ethernet Interface

```bash
# Check ethernet interface name
ifconfig | grep -A 1 "^en[0-9]:"

# Configure static IP (replace en0 with your interface)
sudo ifconfig en0 192.168.100.1 netmask 255.255.255.0
```

### 3. Configure RPI Ethernet Interface (eth0)

```bash
# Create netplan configuration for static IP
wsh rpi "sudo tee /etc/netplan/99-eth0-static.yaml > /dev/null << 'EOF'
network:
  version: 2
  renderer: networkd
  ethernets:
    eth0:
      dhcp4: false
      addresses:
        - 192.168.100.2/24
EOF"

# Apply netplan configuration
wsh rpi "sudo netplan apply"

# Verify eth0 is up
wsh rpi "ip addr show eth0"
```

### 4. Verify Connectivity

```bash
# From Mac: ping RPI
ping -c 3 192.168.100.2

# From RPI: ping Mac
wsh rpi "ping -c 3 192.168.100.1"
```

### 5. Test MACVLAN on eth0

```bash
# Create test MACVLAN network
wsh rpi "sudo docker network create -d macvlan \
  --subnet=192.168.100.0/24 \
  -o parent=eth0 \
  test_macvlan"

# Run test container
wsh rpi "sudo docker run -d --name macvlan_test \
  --network test_macvlan \
  --ip 192.168.100.10 \
  nginx:alpine"

# Verify container is running
wsh rpi "sudo docker ps | grep macvlan_test"

# Test connectivity from Mac
ping -c 3 192.168.100.10

# Clean up
wsh rpi "sudo docker rm -f macvlan_test && sudo docker network rm test_macvlan"
```

### 6. Configure Rodent for MACVLAN on eth0

Update Rodent config at `~/.rodent/rodent.yml.dev`:

```yaml
ad:
  dc:
    enabled: true
    containerName: dc1
    hostname: DC1
    realm: ad.strata.internal
    domain: AD
    networkMode: macvlan          # Use MACVLAN mode
    parentInterface: eth0         # Use eth0 (direct ethernet)
    ipAddress: 192.168.100.10  # IP in direct ethernet subnet
    gateway: ""        # Gateway is not necessary for this direct link
    subnet: 192.168.100.0/24
```

### 7. Run Rodent Service

```bash
wshrod rpi "pkill main; cd ~/rodent; RODENT_CONFIG=/home/rodent/.rodent/rodent.yml.dev go run main.go serve"
```

## Why This Setup Works

### MACVLAN Limitations on WiFi

- Most WiFi access points block multiple MAC addresses from a single wireless client
- This prevents MACVLAN from working on wireless interfaces (wlan0)
- Wired ethernet (eth0) doesn't have this restriction

### Dual Interface Benefits

- **WiFi (wlan0)**: Provides internet connectivity for both devices
- **Ethernet (eth0)**: Provides isolated network for MACVLAN testing
- No conflicts between interfaces (different subnets)

### Routing Behavior

- Default route via wlan0 (192.168.31.1) for internet
- Direct route to 192.168.100.0/24 via eth0
- MACVLAN containers accessible from Mac via eth0 network

## Expected Results

### RPI Network Interfaces

```sh
eth0: 192.168.100.2/24 (static, MACVLAN parent)
wlan0: 192.168.31.116/24 (DHCP, internet)
docker0: 172.17.0.1/16 (default docker bridge)
```

### Routing Table

```sh
default via 192.168.31.1 dev wlan0            # Internet via WiFi
192.168.31.0/24 dev wlan0                     # WiFi subnet
192.168.100.0/24 dev eth0                     # Direct ethernet subnet
172.17.0.0/16 dev docker0                     # Docker bridge
```

### MACVLAN Testing

- ✅ Ping MACVLAN container (192.168.100.10) from Mac: ~1ms latency
- ✅ DNS/LDAP services accessible on MACVLAN IP
- ✅ Internet still works via wlan0

## Troubleshooting

### Cannot ping 192.168.100.2 from Mac

```bash
# Check Mac ethernet IP
ifconfig en0

# Check RPI eth0 status
wsh rpi "ip addr show eth0"
wsh rpi "ip route show"
```

### MACVLAN container not accessible

```bash
# Verify MACVLAN network exists
wsh rpi "sudo docker network ls | grep macvlan"

# Check container IP
wsh rpi "sudo docker inspect <container> | grep IPAddress"

# Verify parent interface
wsh rpi "sudo docker network inspect <network> | grep parent"
```

### Internet not working on RPI

```bash
# Check default route
wsh rpi "ip route show | grep default"

# Should be: default via 192.168.31.1 dev wlan0
# If not, check wlan0 connection
wsh rpi "ip addr show wlan0"
```

## Alternative: Using D-Link Router as Switch

If you need more ethernet ports or want to avoid direct cable connection:

1. Configure D-Link router:
   - Disable DHCP
   - Set static IP: 192.168.31.2
   - Connect only to LAN ports (not WAN)

2. Connect devices:
   - Mac ethernet → D-Link LAN port 1
   - RPI ethernet → D-Link LAN port 2

3. Configure static IPs on same subnet:
   - Mac ethernet: 192.168.100.1/24 (or use DHCP from main router)
   - RPI eth0: 192.168.100.2/24
   - MACVLAN: 192.168.100.10+

This provides a switched ethernet network while maintaining WiFi for internet.
