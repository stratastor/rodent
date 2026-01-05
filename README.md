# Rodent –– API service for securely orchestrating storage of servers at scale

[![Go Report Card](https://goreportcard.com/badge/gojp/goreportcard)](https://goreportcard.com/report/github.com/stratastor/rodent) [![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://github.com/stratastor/rodent/blob/master/LICENSE.txt)

Rodent is a node agent for managing ZFS-based storage systems. It can securely connect to [Strata](https://strata.foo) for centralized management, or run standalone in headless mode. It can also be integrated with other management platforms via its REST and gRPC API.

<img src="https://web-static.strata.foo/landing.png" alt="Strata Platform" width="600" height="400"/>

## Overview

Rodent is a comprehensive agent designed to manage ZFS-based storage systems. It provides a robust set of features for modern Network Attached Storage (NAS) environments:

### Key Features

- **ZFS Pool Management**: Create, import, export, and monitor ZFS pools
- **Dataset Operations**: Manage filesystems, volumes, and snapshots
- **Data Sharing**: Support for SMB/CIFS, NFS(WIP), and iSCSI(WIP) protocols
- **User Management**: Integrated access control and permission management
- **Data Protection**: Snapshot scheduling, replication, and backup workflows
- **Performance Monitoring**: Real-time metrics and health reporting
- **REST API**: Programmatic access to all functionality

Rodent serves as the operational layer between the ZFS subsystem and higher-level storage management interfaces, providing a unified approach to storage administration.

## Quick Start

### Installation

```bash
curl -fsSL https://utils.strata.host/install.sh | sudo bash -s -- --non-interactive
```

See [Installation Guide](docs/INSTALLATION.md) for detailed options.

### Connect to Strata

#### 1. Create a Rodent in Strata

Login to [https://strata.foo](https://strata.foo) and create a new Rodent to obtain your JWT token.

#### 2. Configure Rodent

Edit the configuration file as the `rodent` user:

```bash
sudo -u rodent nano /home/rodent/.rodent/rodent.yml
```

Add your JWT token:

```yaml
ad:
  mode: self-hosted
  adminPassword: Passw0rd
  dc:
    enabled: false
toggle:
  enable: true
  jwt: your-jwt-token-from-strata
  baseurl: https://toggle.strata.foo
  rpcaddr: tunnel.strata.foo:443
```

#### 3. Start Rodent

```bash
sudo systemctl enable --now rodent.service
sudo journalctl -u rodent.service -f
```

Your Rodent will appear as active in Strata.

### Running Manually (Development)

```bash
# Stop the service
sudo systemctl stop rodent.service

# Switch to rodent user and run manually
sudo -u rodent rodent serve
```

Config must be at `~/.rodent/rodent.yml` when running as `rodent` user. Set loglevel in config to `debug` for verbose output.

```yaml
logger:
  loglevel: debug
server:
  loglevel: debug
```

### Active Directory for SMB Shares (Optional)

To enable SMB shares with AD authentication, configure self-hosted or external AD. See [Active Directory Configuration Guide](docs/ACTIVE_DIRECTORY.md).

## Documentation

- [Installation Guide](docs/INSTALLATION.md)
- [Active Directory Setup](docs/ACTIVE_DIRECTORY.md)

### Common First Issues

Look for issues labeled with:

- `good first issue`: Beginner-friendly tasks
- `Needs Fix`: Known issues needing attention
- `tests`: Test coverage improvements
- `Essential`: It is understood that this can't be overlooked, and demands attention.

- Adding test cases in data_transfer_test.go, dataset_test.go, pool_test.go and other pivotal modules
- Improving error messages in errors/types.go
- Adding documentation examples in docs/ (/notes?)
- Implementing missing validations

### Contribution Workflow

Have a look at [Contributing Guide](./CONTRIBUTING.md), [Code of Conduct](./CODE_OF_CONDUCT.md) and [Pull Request Guidelines](./PULL_REQUEST_GUIDELINES.md)

- Fork repository
- Create feature branch
- Make changes
- Run tests
- Run linter, go fmt
- Commit changes
- Submit PR

### Additional Resources

- [OpenZFS Docs](https://openzfs.github.io/openzfs-docs/index.html)
- [ZFS Man pages](https://openzfs.github.io/openzfs-docs/index.html)

## License

This project is licensed under the **Apache License 2.0**. By using this software, you are granted:

- **Freedom to Use**: You can use the software for personal, academic, or commercial purposes.
- **Freedom to Modify**: You can modify the code to suit your needs.
- **Freedom to Distribute**: You can distribute the software, including any modifications you make.
- **Freedom from Patent Worries**: Contributors provide a broad patent license, protecting you from patent claims related to the software.

### Summary of Key Terms

1. **Attribution**: You must include the original license and copyright notice in any distribution.
2. **No Liability**: The software is provided "as is," without warranties or guarantees.
3. **Patent Protection**: The license includes a patent grant, and your rights are terminated if you initiate patent litigation involving the software.

For the full text of the license, see the [LICENSE](./LICENSE.txt) file.
