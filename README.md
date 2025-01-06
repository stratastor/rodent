# Rodent [WIP]

StrataSTOR Node Agent for ZFS management(primarily) but it seems as though it'll have to go beyond the call of duty to work the underlying system.

## Overview

Rodent is a ZFS management agent that provides:

- RESTful API for ZFS operations
- Dataset and pool management
- Remote data transfer capabilities
- Health monitoring
- Configuration management

## Development

**‼️ Care to try these only in staging environments as `sudo` privileges are required for ZFS calls.**

### Installation

```bash
go install github.com/stratastor/rodent@latest
```

### Code Organization

```sh
rodent/
├── cmd/                    # Command line interface
├── config/                 # Error definitions
├── pkg/           
│   ├── errors/            # Error definitions
│   ├── health/           # Health checks
│   ├── lifecycle/        # Process lifecycle
│   └── zfs/              # ZFS operations
│       ├── api/          # REST API
│       ├── dataset/      # Dataset operations
│       ├── pool/         # Pool operations
│       └── command/      # Command execution
├── notes/                # Design documents
```

### Misc. Commands

```bash
sudo go run main.go serve 
```

```bash
ubuntu@staging:~/rodent/pkg/zfs$ rodent -h
Rodent: StrataSTOR Node Agent

Usage:
  rodent [command]

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  config      Manage Rodent configuration
  health      Check Rodent health
  help        Help about any command
  logs        View Rodent server logs
  serve       Start the Rodent server
  status      Check Rodent server status
  version     Show Rodent version

Flags:
  -h, --help   help for rodent

Use "rodent [command] --help" for more information about a command.
```

Place rodent.yml config in the root level of the directory. Alternatively, let `rodent` generate defaults. `rodent config print` will print config details.

```bash
ubuntu@staging:~/rodent/pkg/zfs$ rodent config  print
# Configuration loaded from: /home/ubuntu/.rodent/rodent.yml
---
server:
  port: 8042
  loglevel: info
  daemonize: false
health:
  interval: 30s
  endpoint: /health
logs:
  path: /var/log/rodent/rodent.log
  retention: 7d
  output: stdout
logger:
  loglevel: info
  enablesentry: false
  sentrydsn: ""
environment: dev
```

### Testing

`cd` to individual modules and run individual test suite; better than running everything in one go.

```bash
ubuntu@staging:~/rodent$ cd pkg/zfs/dataset/
ubuntu@staging:~/rodent/pkg/zfs/dataset$ sudo go test -v -run TestDatasetOperations
--- PASS: TestDatasetOperations (3.05s)
    --- PASS: TestDatasetOperations/Filesystems (0.74s)
        --- PASS: TestDatasetOperations/Filesystems/Create (0.14s)
        --- PASS: TestDatasetOperations/Filesystems/Properties (0.06s)
        --- PASS: TestDatasetOperations/Filesystems/Snapshots (0.11s)
        --- PASS: TestDatasetOperations/Filesystems/Clones (0.10s)
        --- PASS: TestDatasetOperations/Filesystems/Inherit (0.02s)
        --- PASS: TestDatasetOperations/Filesystems/Mount (0.17s)
        --- PASS: TestDatasetOperations/Filesystems/Rename (0.07s)
        --- PASS: TestDatasetOperations/Filesystems/Destroy (0.06s)
    --- PASS: TestDatasetOperations/Volumes (0.43s)
        --- PASS: TestDatasetOperations/Volumes/CreateVolume (0.13s)
        --- PASS: TestDatasetOperations/Volumes/CreateSparseVolume (0.08s)
        --- PASS: TestDatasetOperations/Volumes/CreateVolumeWithParent (0.22s)
    --- PASS: TestDatasetOperations/DiffOperations (0.45s)
        --- PASS: TestDatasetOperations/DiffOperations/SnapshotDiff (0.03s)
        --- PASS: TestDatasetOperations/DiffOperations/FileModification (0.10s)
        --- PASS: TestDatasetOperations/DiffOperations/RenameOperation (0.07s)
        --- PASS: TestDatasetOperations/DiffOperations/ErrorCases (0.01s)
            --- PASS: TestDatasetOperations/DiffOperations/ErrorCases/missing_names (0.00s)
            --- PASS: TestDatasetOperations/DiffOperations/ErrorCases/single_name (0.00s)
            --- PASS: TestDatasetOperations/DiffOperations/ErrorCases/non-existent_snapshot (0.01s)
    --- PASS: TestDatasetOperations/ShareOperations (1.04s)
        --- SKIP: TestDatasetOperations/ShareOperations/ShareDataset (0.03s)
        --- PASS: TestDatasetOperations/ShareOperations/ShareAll (0.35s)
        --- PASS: TestDatasetOperations/ShareOperations/UnshareDataset (0.09s)
        --- PASS: TestDatasetOperations/ShareOperations/UnshareAll (0.10s)
        --- PASS: TestDatasetOperations/ShareOperations/ErrorCases (0.00s)
PASS
ok      github.com/stratastor/rodent/pkg/zfs/dataset    3.058s
```

```bash
ubuntu@staging:~/rodent$ cd pkg/zfs && sudo go test -v ./...
```

### Setup

- Ubuntu 24.04
- Go 1.23+
- nfs and samba
- zfs-2.3.0-rc4

### ZFS Package

The zfs package provides core ZFS functionality:

- Dataset operations (create, destroy, snapshot)
- Pool management (import, export, status)
- Property handling
- Remote transfers
- Command execution safety

```go
// Dataset operations
pkg/zfs/dataset/dataset.go

// Pool management
pkg/zfs/pool/pool.go
```

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
- Run linter
- Commit changes
- Submit PR

### Additional Resources

- [Run-along development blog](https://puckish.life)
- [OpenZFS Docs](https://openzfs.github.io/openzfs-docs/index.html)
- [ZFS Man pages](https://openzfs.github.io/openzfs-docs/index.html)
- [Notes](./notes)

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
