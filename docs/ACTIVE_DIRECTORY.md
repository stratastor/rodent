# Active Directory Configuration Guide

## Overview

Rodent uses Samba AD DC for SMB share authentication. You can either run a self-hosted AD DC in a Docker container or join an external AD domain. Self-hosted mode is recommended unless you have an existing AD infrastructure.

## TL;DR

**Cloud VMs (AWS/GCP/Azure)**: Use `host` network mode with a dedicated network interface
**Physical servers / VMware**: Use `macvlan` network mode on your existing ethernet interface
**Development (RPi5 with WiFi+Ethernet)**: Use `macvlan` on ethernet with WiFi for internet

**Important:** MACVLAN does not require an additional NIC. It works on your existing ethernet interface.

## Network Modes Explained

### Host Mode (Cloud-Friendly)

The AD DC container shares the host's network stack and binds to a dedicated physical interface.

**When to use:**

- Cloud instances (AWS EC2, GCP Compute, Azure VM)
- Environments where MACVLAN is not supported
- Virtual machines without bridge networking support

**Requirements:**

- A dedicated network interface (secondary ENI/NIC) with a static IP
- Rodent automatically configures the interface via netplan when starting

**How it works:**

```text
Cloud Network (e.g., 10.0.1.0/24)
     |
     +-- Host Primary NIC (10.0.1.10) → Internet, SSH, Rodent API, SMB/NFS Shares
     |
     +-- Host Secondary NIC (10.0.1.20) → AD DC services only
```

### MACVLAN Mode (Physical/Performance)

The AD DC container gets its own MAC address and appears as a separate device on the network.

**When to use:**

- Physical servers
- VMware / VirtualBox VMs
- Development environments with direct ethernet access

**Requirements:**

- Your existing ethernet interface (no additional NIC needed!)
- Available IP address in the same subnet as your ethernet interface
- Rodent automatically creates the macvlan-shim interface for host-container communication

**How it works:**

```text
Physical Network (192.168.1.0/24)
      |
   [eth0] (192.168.1.10)          ← Your existing ethernet interface
      |
      +-- [macvlan-shim] (.253)   ← Auto-created by Rodent
      |
      +-- [DC Container] (.20)    ← AD DC with its own MAC/IP
```

The shim interface is required because Linux prevents direct communication between the host and MACVLAN containers on the same parent interface. **Rodent creates this automatically** when starting the AD DC.

## Configuration Examples

### Cloud Instance (Host Mode)

#### Step 1: Attach a secondary network interface

In your cloud console (AWS/GCP/Azure), attach a secondary ENI to your instance. Note:

- The interface name (e.g., `ens6` on AWS)
- The IP address assigned to it (e.g., `10.0.1.20`)

**Step 2: Configure security group** (AWS example)

Create/modify security group for the secondary ENI:

```text
Inbound Rules:
- All TCP/UDP from self (sg-xxxxxx)  → Allows AD DC ports
```

This allows the host (primary NIC) to reach AD DC services on the secondary NIC.

**Step 3: Edit Rodent config** (as `rodent` user)

```bash
sudo -u rodent nano /home/rodent/.rodent/rodent.yml
```

```yaml
ad:
  mode: self-hosted
  adminpassword: YourSecurePassword123!
  realm: ad.mycompany.internal
  dc:
    enabled: true
    containerName: dc1
    hostname: DC1
    realm: ad.mycompany.internal
    domain: MYCOMPANY
    autoJoin: true
    networkMode: host
    parentInterface: ens6        # Secondary ENI name
    ipAddress: 10.0.1.20         # Secondary ENI IP (no CIDR)
    subnet: 10.0.1.0/24

shares:
  smb:
    realm: ad.mycompany.internal
    workgroup: MYCOMPANY
```

**Step 4: Start Rodent** (as `ubuntu` user)

```bash
sudo systemctl start rodent.service
sudo journalctl -u rodent.service -f
```

Rodent will automatically configure netplan for `ens6` when starting.

### Physical Server / VMware (MACVLAN Mode)

#### Step 1: Identify your ethernet interface

```bash
ip link show
```

Look for your ethernet interface (e.g., `eth0`, `eno1`, `enp0s3`). You'll use this existing interface - no need to add another NIC.

#### Step 2: Pick an available IP

Choose an IP address on the same subnet as your ethernet interface. For example, if your host is `192.168.1.10`, you might use `192.168.1.20` for the AD DC.

#### Step 3: Edit Rodent config** (as `rodent` user)

```bash
sudo -u rodent nano /home/rodent/.rodent/rodent.yml
```

```yaml
ad:
  mode: self-hosted
  adminpassword: YourSecurePassword123!
  realm: ad.mycompany.internal
  dc:
    enabled: true
    containerName: dc1
    hostname: DC1
    realm: ad.mycompany.internal
    domain: MYCOMPANY
    autoJoin: true
    networkMode: macvlan
    parentInterface: eth0            # Your existing ethernet interface
    ipAddress: 192.168.1.20          # Available IP on your network
    gateway: 192.168.1.1             # Your network gateway (optional)
    subnet: 192.168.1.0/24           # Your network subnet
    shimIP: 192.168.1.253            # Auto-assigned if omitted

shares:
  smb:
    realm: ad.mycompany.internal
    workgroup: MYCOMPANY
```

**Gateway vs ShimIP:**

- `gateway`: The actual network gateway for routing to external networks (usually your router).

- `shimIP`: The IP for the macvlan-shim interface that Rodent creates automatically. If omitted, Rodent assigns `.253` in your subnet.

**Step 4: Start Rodent** (as `ubuntu` user)

```bash
sudo systemctl start rodent.service
sudo journalctl -u rodent.service -f
```

Watch for `macvlan-shim interface created successfully` in the logs.

### Development (RPi5 / Dual-NIC Setup)

If you have both WiFi and ethernet, use WiFi for internet and ethernet for MACVLAN. This allows MACVLAN testing without disrupting internet connectivity.

See [MACVLAN Direct Ethernet Setup](../notes/macvlan-direct-ethernet-setup.md) for detailed dual-NIC configuration with direct ethernet connection.

#### Step 1: Configure ethernet with static IP

```bash
sudo tee /etc/netplan/99-eth0-static.yaml > /dev/null << 'EOF'
network:
  version: 2
  renderer: networkd
  ethernets:
    eth0:
      dhcp4: false
      addresses:
        - 192.168.100.2/24
EOF

sudo netplan apply
```

#### Step 2: Edit Rodent config

```yaml
ad:
  mode: self-hosted
  adminpassword: DevPassword123!
  realm: ad.strata.internal
  dc:
    enabled: true
    containerName: dc1
    hostname: DC1
    realm: ad.strata.internal
    domain: AD
    autoJoin: true
    networkMode: macvlan
    parentInterface: eth0
    ipAddress: 192.168.100.10/24
    gateway: 192.168.100.1
    subnet: 192.168.100.0/24
    shimIP: 192.168.100.253

shares:
  smb:
    realm: ad.strata.internal
    workgroup: AD
```

## External AD (Enterprise)

If you already have an AD infrastructure, configure Rodent to join it instead of running its own DC.

```yaml
ad:
  mode: external
  adminpassword: DomainAdminPassword
  realm: CORP.EXAMPLE.COM
  external:
    domainControllers:
      - dc1.corp.example.com
      - dc2.corp.example.com  # Failover DC
    adminUser: Administrator
    autoJoin: true

shares:
  smb:
    realm: CORP.EXAMPLE.COM
    workgroup: CORP
```

Rodent will try each DC in order until one succeeds (automatic failover).

## Configuration Fields Reference

### Required Fields

| Field | Description | Example |
|-------|-------------|---------|
| `ad.mode` | `self-hosted` or `external` | `self-hosted` |
| `ad.adminpassword` | AD administrator password | `Passw0rd!` |
| `ad.realm` | Kerberos realm (uppercase) | `AD.STRATA.INTERNAL` |
| `ad.dc.hostname` | DC hostname (uppercase) | `DC1` |
| `ad.dc.domain` | NetBIOS domain name | `AD` |
| `ad.dc.networkMode` | `host` or `macvlan` | `macvlan` |
| `ad.dc.parentInterface` | Interface name | `eth0` |
| `ad.dc.ipAddress` | IP for AD DC (CIDR for macvlan, no CIDR for host) | `192.168.1.20/24` |
| `ad.dc.subnet` | Network subnet | `192.168.1.0/24` |
| `shares.smb.realm` | SMB realm (matches ad.realm) | `AD.STRATA.INTERNAL` |
| `shares.smb.workgroup` | NetBIOS workgroup name | `AD` |

### Optional Fields

| Field | Description | Default |
|-------|-------------|---------|
| `ad.dc.gateway` | Network gateway | None (not needed for direct connections) |
| `ad.dc.shimIP` | MACVLAN shim IP (macvlan only) | Auto-assigned to `.253` |
| `ad.dc.autoJoin` | Auto-join domain on startup | `true` |
| `ad.dc.dnsForwarder` | Upstream DNS for internet resolution | `8.8.8.8` |
| `ad.dc.containerName` | Docker container name | `dc1` |

## Starting Rodent

After editing the config:

```bash
# Start the service (as ubuntu user)
sudo systemctl start rodent.service

# Watch the logs
sudo journalctl -u rodent.service -f
```

**What happens:**

1. **Host mode**: Rodent configures netplan for the secondary interface automatically
2. **MACVLAN mode**: Rodent creates the macvlan-shim interface automatically
3. AD DC container starts with configured network mode
4. Rodent waits for DC to be ready (checks LDAPS port 636)
5. Kerberos and NSS are configured
6. Host joins the domain automatically (if `autoJoin: true`)
7. Winbind service restarts

**Success indicators in logs:**

```text
AD DC service started successfully
macvlan-shim interface created successfully  (macvlan mode only)
Waiting for AD DC to be ready...
AD DC is ready
Configuring Kerberos...
Configuring NSS...
Joining domain...
Domain join successful
Winbind service restarted
```

## Verifying Domain Join

```bash
# Check domain membership
sudo wbinfo --ping-dc

# List AD users
sudo wbinfo -u

# Test user lookup
sudo id Administrator

# Check Kerberos ticket
sudo kinit Administrator
sudo klist
```

## Security Notes

### Password Requirements

Use strong passwords for `adminpassword`:

- Minimum 8 characters
- Mixed case letters, numbers, symbols
- Avoid dictionary words

### Cloud Security Groups (AWS Example)

For host mode in cloud environments, create a security group for the AD DC secondary ENI with specific port rules.

**Create Security Group:**

```bash
# Set your VPC ID
VPC_ID="vpc-xxxxxxxx"

# Create security group
SG_ID=$(aws ec2 create-security-group \
  --group-name SambaADDC \
  --description "Security group for Samba AD DC" \
  --vpc-id $VPC_ID \
  --query 'GroupId' \
  --output text)

echo "Created security group: $SG_ID"

# Add inbound rules (self-referencing for host-to-DC communication)
aws ec2 authorize-security-group-ingress --group-id $SG_ID --ip-permissions IpProtocol=tcp,FromPort=53,ToPort=53,UserIdGroupPairs="[{GroupId=$SG_ID,Description='DNS'}]"
aws ec2 authorize-security-group-ingress --group-id $SG_ID --ip-permissions IpProtocol=udp,FromPort=53,ToPort=53,UserIdGroupPairs="[{GroupId=$SG_ID,Description='DNS'}]"
aws ec2 authorize-security-group-ingress --group-id $SG_ID --ip-permissions IpProtocol=tcp,FromPort=88,ToPort=88,UserIdGroupPairs="[{GroupId=$SG_ID,Description='Kerberos'}]"
aws ec2 authorize-security-group-ingress --group-id $SG_ID --ip-permissions IpProtocol=udp,FromPort=88,ToPort=88,UserIdGroupPairs="[{GroupId=$SG_ID,Description='Kerberos'}]"
aws ec2 authorize-security-group-ingress --group-id $SG_ID --ip-permissions IpProtocol=udp,FromPort=123,ToPort=123,UserIdGroupPairs="[{GroupId=$SG_ID,Description='NTP'}]"
aws ec2 authorize-security-group-ingress --group-id $SG_ID --ip-permissions IpProtocol=tcp,FromPort=135,ToPort=135,UserIdGroupPairs="[{GroupId=$SG_ID,Description='End Point Mapper (DCE/RPC Locator Service)'}]"
aws ec2 authorize-security-group-ingress --group-id $SG_ID --ip-permissions IpProtocol=udp,FromPort=137,ToPort=137,UserIdGroupPairs="[{GroupId=$SG_ID,Description='NetBIOS Name Service'}]"
aws ec2 authorize-security-group-ingress --group-id $SG_ID --ip-permissions IpProtocol=udp,FromPort=138,ToPort=138,UserIdGroupPairs="[{GroupId=$SG_ID,Description='NetBIOS Datagram'}]"
aws ec2 authorize-security-group-ingress --group-id $SG_ID --ip-permissions IpProtocol=tcp,FromPort=389,ToPort=389,UserIdGroupPairs="[{GroupId=$SG_ID,Description='LDAP'}]"
aws ec2 authorize-security-group-ingress --group-id $SG_ID --ip-permissions IpProtocol=udp,FromPort=389,ToPort=389,UserIdGroupPairs="[{GroupId=$SG_ID,Description='LDAP'}]"
aws ec2 authorize-security-group-ingress --group-id $SG_ID --ip-permissions IpProtocol=tcp,FromPort=445,ToPort=445,UserIdGroupPairs="[{GroupId=$SG_ID,Description='SMB over TCP'}]"
aws ec2 authorize-security-group-ingress --group-id $SG_ID --ip-permissions IpProtocol=tcp,FromPort=464,ToPort=464,UserIdGroupPairs="[{GroupId=$SG_ID,Description='Kerberos kpasswd'}]"
aws ec2 authorize-security-group-ingress --group-id $SG_ID --ip-permissions IpProtocol=udp,FromPort=464,ToPort=464,UserIdGroupPairs="[{GroupId=$SG_ID,Description='Kerberos kpasswd'}]"
aws ec2 authorize-security-group-ingress --group-id $SG_ID --ip-permissions IpProtocol=tcp,FromPort=636,ToPort=636,UserIdGroupPairs="[{GroupId=$SG_ID,Description='LDAPS'}]"
aws ec2 authorize-security-group-ingress --group-id $SG_ID --ip-permissions IpProtocol=tcp,FromPort=3268,ToPort=3268,UserIdGroupPairs="[{GroupId=$SG_ID,Description='Global Catalog'}]"
aws ec2 authorize-security-group-ingress --group-id $SG_ID --ip-permissions IpProtocol=tcp,FromPort=3269,ToPort=3269,UserIdGroupPairs="[{GroupId=$SG_ID,Description='Global Catalog SSL'}]"
aws ec2 authorize-security-group-ingress --group-id $SG_ID --ip-permissions IpProtocol=tcp,FromPort=50000,ToPort=55000,UserIdGroupPairs="[{GroupId=$SG_ID,Description='Dynamic RPC Ports'}]"

echo "Added all AD DC inbound rules"
```

**Apply Security Group:**

Attach the security group to the secondary ENI dedicated for AD DC:

```bash
# Get your secondary ENI ID
ENI_ID="eni-xxxxxxxx"

# Attach security group
aws ec2 modify-network-interface-attribute \
  --network-interface-id $ENI_ID \
  --groups $SG_ID
```

**For SMB Share Access:**

If other machines need to mount SMB shares from the host, allow port 445 on the **primary** ENI's security group:

```bash
# Add SMB port to primary ENI security group (adjust source as needed)
PRIMARY_SG_ID="sg-xxxxxxxx"

# Allow from specific subnet
aws ec2 authorize-security-group-ingress \
  --group-id $PRIMARY_SG_ID \
  --protocol tcp \
  --port 445 \
  --cidr 10.0.0.0/16

# Or allow from specific security group
aws ec2 authorize-security-group-ingress \
  --group-id $PRIMARY_SG_ID \
  --protocol tcp \
  --port 445 \
  --source-group sg-client-xxxxxxxx
```

The self-referencing rules (`--source-group $SG_ID`) allow the host (via its secondary ENI) to communicate with the AD DC on the same ENI. Outbound rules are open by default (0.0.0.0/0).

### Production Considerations

- Use proper DNS infrastructure instead of relying on `dnsForwarder`
- AD DC binds to all services (DNS, LDAP, Kerberos, SMB)
- For internet-facing servers, restrict access to AD DC ports
- Consider using a separate VLAN for AD traffic in enterprise environments

## Network Port Requirements

The AD DC uses these ports. For cloud environments, see the AWS CLI commands above to configure security groups automatically.

| Port | Protocol | Service | Description |
|------|----------|---------|-------------|
| 53 | TCP/UDP | DNS | Domain name resolution |
| 88 | TCP/UDP | Kerberos | Authentication |
| 123 | UDP | NTP | Time synchronization |
| 135 | TCP | RPC Endpoint Mapper | DCE/RPC Locator Service |
| 137 | UDP | NetBIOS Name Service | Name registration and resolution |
| 138 | UDP | NetBIOS Datagram | Datagram distribution |
| 389 | TCP/UDP | LDAP | Directory access |
| 445 | TCP | SMB | File sharing |
| 464 | TCP/UDP | Kerberos kpasswd | Password changes |
| 636 | TCP | LDAPS | Secure LDAP (used for health checks) |
| 3268 | TCP | Global Catalog | Directory-wide searches |
| 3269 | TCP | Global Catalog SSL | Secure global catalog |
| 50000-55000 | TCP | Dynamic RPC | Samba dynamic ports |

**For cloud environments:** Use the AWS CLI commands in the Security Groups section to create the security group with all required ports. The self-referencing rules allow host-to-DC communication via different ENIs on the same instance.

**For SMB shares:** Additionally allow port 445 on the primary ENI for clients mounting shares.

## Troubleshooting

### AD DC container won't start

```bash
# Check Docker logs
sudo docker logs dc1

# Check network interface exists
ip link show eth0

# For host mode: verify secondary interface
ip addr show ens6
```

### Cannot ping AD DC IP

**MACVLAN mode:**

```bash
# Check if shim interface was created
ip addr show macvlan-shim

# Should show something like:
# inet 192.168.1.253/24 scope global macvlan-shim

# If missing, check Rodent logs for errors:
sudo journalctl -u rodent.service | grep macvlan-shim
```

**Host mode:**

```bash
# Verify secondary interface is configured by Rodent
ip addr show ens6

# Check that AD DC is binding to correct IP
sudo netstat -tlnp | grep :636
```

### Domain join fails

```bash
# Check Kerberos config
cat /etc/krb5.conf

# Test kinit manually
sudo kinit Administrator@AD.STRATA.INTERNAL

# Check DC is reachable
sudo ldapsearch -x -H ldaps://192.168.1.20 -b "DC=ad,DC=strata,DC=internal"

# Check winbind
sudo systemctl status winbind
```

### MACVLAN not working on WiFi

WiFi access points typically block multiple MAC addresses from a single client. Use ethernet for MACVLAN or switch to host mode.

For development with dual-NIC setups, see the [RPi5 MACVLAN guide](../notes/macvlan-direct-ethernet-setup.md).

## Further Reading

- [Samba AD DC Documentation](https://wiki.samba.org/index.php/Setting_up_Samba_as_an_Active_Directory_Domain_Controller)
- [MACVLAN Networking](https://docs.docker.com/network/macvlan/)
- [Netplan Configuration](https://netplan.io/reference/)
- [RPi5 MACVLAN Setup](../notes/macvlan-direct-ethernet-setup.md) - Detailed guide for dual-NIC development environments
