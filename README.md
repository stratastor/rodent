# Rodent [WIP]

[![Go Report Card](https://goreportcard.com/badge/gojp/goreportcard)](https://goreportcard.com/report/github.com/stratastor/rodent) [![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://github.com/stratastor/rodent/blob/master/LICENSE.txt)

StrataSTOR node agent for ZFS operations(primarily).

## Overview

Rodent is a ZFS management agent that provides:

- HTTP API for ZFS operations
- Dataset and pool management
- Remote data transfer capabilities
- Health monitoring
- Configuration management

## Development

**‼️ Caution: Mind the `sudo` usage! Try these only in staging environments as `sudo` privileges are required for ZFS calls.**

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

`cd` to individual modules and run necessary test suite; better than running everything in one go.

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

- Dataset operations (create, destroy, snapshot, and others...)
- Pool management (import, export, status, and...)
- HTTP API handlers
- Property handling
- Remote transfers
- Command execution safety

```go
// Dataset operations
pkg/zfs/dataset/dataset.go

// Pool management
pkg/zfs/pool/pool.go
```

### HTTP API

[HTTP Routes](./pkg/zfs/api/routes.go) are listed in the routes.go file and the request payload schema is scattered across [pkg/zfs/dataset/types.go](pkg/zfs/dataset/types.go), [pkg/zfs/pool/types.go](pkg/zfs/pool/types.go) and [pkg/zfs/api/types.go](pkg/zfs/api/types.go) files.

🙋 TODO: API documentation

Unlike Pool operations, Dataset API maynot be RESTFUL. Having dataset values with "/" in the URI params is inconvenient and may lead to confusion. Hence, we will pass information in the body to keep the URI clean and simple.

Here's a gist but the recommended source of truth is the ./pkg/zfs/api/routes.go file:

```go
[GIN-debug] GET    /health                   --> github.com/stratastor/rodent/pkg/server.Start.func1 (3 handlers)
[GIN-debug] POST   /api/v1/rodent/zfs/dataset/list --> github.com/stratastor/rodent/pkg/zfs/api.(*DatasetHandler).listDatasets-fm (4 handlers)
[GIN-debug] POST   /api/v1/rodent/zfs/dataset/delete --> github.com/stratastor/rodent/pkg/zfs/api.(*DatasetHandler).destroyDataset-fm (5 handlers)
[GIN-debug] POST   /api/v1/rodent/zfs/dataset/rename --> github.com/stratastor/rodent/pkg/zfs/api.(*DatasetHandler).renameDataset-fm (5 handlers)
[GIN-debug] POST   /api/v1/rodent/zfs/dataset/diff --> github.com/stratastor/rodent/pkg/zfs/api.(*DatasetHandler).diffDataset-fm (5 handlers)
[GIN-debug] POST   /api/v1/rodent/zfs/dataset/properties/list --> github.com/stratastor/rodent/pkg/zfs/api.(*DatasetHandler).listProperties-fm (5 handlers)
[GIN-debug] POST   /api/v1/rodent/zfs/dataset/property/fetch --> github.com/stratastor/rodent/pkg/zfs/api.(*DatasetHandler).getProperty-fm (6 handlers)
[GIN-debug] POST   /api/v1/rodent/zfs/dataset/property --> github.com/stratastor/rodent/pkg/zfs/api.(*DatasetHandler).setProperty-fm (6 handlers)
[GIN-debug] POST   /api/v1/rodent/zfs/dataset/property/inherit --> github.com/stratastor/rodent/pkg/zfs/api.(*DatasetHandler).inheritProperty-fm (6 handlers)
[GIN-debug] POST   /api/v1/rodent/zfs/dataset/filesystems/list --> github.com/stratastor/rodent/pkg/zfs/api.(*DatasetHandler).listFilesystems-fm (4 handlers)
[GIN-debug] POST   /api/v1/rodent/zfs/dataset/filesystem --> github.com/stratastor/rodent/pkg/zfs/api.(*DatasetHandler).createFilesystem-fm (6 handlers)
[GIN-debug] POST   /api/v1/rodent/zfs/dataset/filesystem/mount --> github.com/stratastor/rodent/pkg/zfs/api.(*DatasetHandler).mountDataset-fm (6 handlers)
[GIN-debug] POST   /api/v1/rodent/zfs/dataset/filesystem/unmount --> github.com/stratastor/rodent/pkg/zfs/api.(*DatasetHandler).unmountDataset-fm (5 handlers)
[GIN-debug] POST   /api/v1/rodent/zfs/dataset/volumes/list --> github.com/stratastor/rodent/pkg/zfs/api.(*DatasetHandler).listVolumes-fm (4 handlers)
[GIN-debug] POST   /api/v1/rodent/zfs/dataset/volume --> github.com/stratastor/rodent/pkg/zfs/api.(*DatasetHandler).createVolume-fm (7 handlers)
[GIN-debug] POST   /api/v1/rodent/zfs/dataset/snapshots/list --> github.com/stratastor/rodent/pkg/zfs/api.(*DatasetHandler).listSnapshots-fm (4 handlers)
[GIN-debug] POST   /api/v1/rodent/zfs/dataset/snapshot --> github.com/stratastor/rodent/pkg/zfs/api.(*DatasetHandler).createSnapshot-fm (6 handlers)
[GIN-debug] POST   /api/v1/rodent/zfs/dataset/snapshot/rollback --> github.com/stratastor/rodent/pkg/zfs/api.(*DatasetHandler).rollbackSnapshot-fm (5 handlers)
[GIN-debug] POST   /api/v1/rodent/zfs/dataset/clone --> github.com/stratastor/rodent/pkg/zfs/api.(*DatasetHandler).createClone-fm (7 handlers)
[GIN-debug] POST   /api/v1/rodent/zfs/dataset/clone/promote --> github.com/stratastor/rodent/pkg/zfs/api.(*DatasetHandler).promoteClone-fm (5 handlers)
[GIN-debug] POST   /api/v1/rodent/zfs/dataset/bookmarks/list --> github.com/stratastor/rodent/pkg/zfs/api.(*DatasetHandler).listBookmarks-fm (4 handlers)
[GIN-debug] POST   /api/v1/rodent/zfs/dataset/bookmark --> github.com/stratastor/rodent/pkg/zfs/api.(*DatasetHandler).createBookmark-fm (5 handlers)
[GIN-debug] POST   /api/v1/rodent/zfs/dataset/permissions/list --> github.com/stratastor/rodent/pkg/zfs/api.(*DatasetHandler).listPermissions-fm (5 handlers)
[GIN-debug] POST   /api/v1/rodent/zfs/dataset/permissions --> github.com/stratastor/rodent/pkg/zfs/api.(*DatasetHandler).allowPermissions-fm (6 handlers)
[GIN-debug] DELETE /api/v1/rodent/zfs/dataset/permissions --> github.com/stratastor/rodent/pkg/zfs/api.(*DatasetHandler).unallowPermissions-fm (6 handlers)
[GIN-debug] POST   /api/v1/rodent/zfs/dataset/share --> github.com/stratastor/rodent/pkg/zfs/api.(*DatasetHandler).shareDataset-fm (4 handlers)
[GIN-debug] DELETE /api/v1/rodent/zfs/dataset/share --> github.com/stratastor/rodent/pkg/zfs/api.(*DatasetHandler).unshareDataset-fm (4 handlers)
[GIN-debug] POST   /api/v1/rodent/zfs/dataset/transfer/send --> github.com/stratastor/rodent/pkg/zfs/api.(*DatasetHandler).sendDataset-fm (4 handlers)
[GIN-debug] POST   /api/v1/rodent/zfs/dataset/transfer/resume-token/fetch --> github.com/stratastor/rodent/pkg/zfs/api.(*DatasetHandler).getResumeToken-fm (5 handlers)
[GIN-debug] POST   /api/v1/rodent/zfs/pools  --> github.com/stratastor/rodent/pkg/zfs/api.(*PoolHandler).createPool-fm (8 handlers)
[GIN-debug] GET    /api/v1/rodent/zfs/pools  --> github.com/stratastor/rodent/pkg/zfs/api.(*PoolHandler).listPools-fm (4 handlers)
[GIN-debug] DELETE /api/v1/rodent/zfs/pools/:name --> github.com/stratastor/rodent/pkg/zfs/api.(*PoolHandler).destroyPool-fm (5 handlers)
[GIN-debug] POST   /api/v1/rodent/zfs/pools/import --> github.com/stratastor/rodent/pkg/zfs/api.(*PoolHandler).importPool-fm (5 handlers)
[GIN-debug] POST   /api/v1/rodent/zfs/pools/:name/export --> github.com/stratastor/rodent/pkg/zfs/api.(*PoolHandler).exportPool-fm (5 handlers)
[GIN-debug] GET    /api/v1/rodent/zfs/pools/:name/status --> github.com/stratastor/rodent/pkg/zfs/api.(*PoolHandler).getPoolStatus-fm (5 handlers)
[GIN-debug] GET    /api/v1/rodent/zfs/pools/:name/properties --> github.com/stratastor/rodent/pkg/zfs/api.(*PoolHandler).getProperties-fm (5 handlers)
[GIN-debug] GET    /api/v1/rodent/zfs/pools/:name/properties/:property --> github.com/stratastor/rodent/pkg/zfs/api.(*PoolHandler).getProperty-fm (6 handlers)
[GIN-debug] PUT    /api/v1/rodent/zfs/pools/:name/properties/:property --> github.com/stratastor/rodent/pkg/zfs/api.(*PoolHandler).setProperty-fm (7 handlers)
[GIN-debug] POST   /api/v1/rodent/zfs/pools/:name/scrub --> github.com/stratastor/rodent/pkg/zfs/api.(*PoolHandler).scrubPool-fm (5 handlers)
[GIN-debug] POST   /api/v1/rodent/zfs/pools/:name/resilver --> github.com/stratastor/rodent/pkg/zfs/api.(*PoolHandler).resilverPool-fm (5 handlers)
[GIN-debug] POST   /api/v1/rodent/zfs/pools/:name/devices/attach --> github.com/stratastor/rodent/pkg/zfs/api.(*PoolHandler).attachDevice-fm (5 handlers)
[GIN-debug] POST   /api/v1/rodent/zfs/pools/:name/devices/detach --> github.com/stratastor/rodent/pkg/zfs/api.(*PoolHandler).detachDevice-fm (5 handlers)
[GIN-debug] POST   /api/v1/rodent/zfs/pools/:name/devices/replace --> github.com/stratastor/rodent/pkg/zfs/api.(*PoolHandler).replaceDevice-fm (5 handlers)

```

[API test cases](./pkg/zfs/api/dataset_test.go) provides reference usage but perhaps `curl` commands might illustrate it cleaner.

Assuming zfs pool `tpool` is already created, and available, try the following:

### 1. Run rodent server

```sh
sudo go run main.go serve
```

### 2. Create request data

```sh
cat <<EOF > create.json
{
    "name": "tpool/ds1",
    "dry_run": false,
    "verbose": true,
    "parsable": true
}
EOF
```

```sh
cat <<EOF > list.json
{
    "name": "tpool/ds1",
    "type": "all",
    "recursive": true
}
EOF
```

### 3. Make the `curl` requests

```sh
curl -s -S --json @create.json -X POST http://localhost:8042/api/v1/dataset/filesystem | jq
```

```sh
curl -s -S --json @list.json -X GET http://localhost:8042/api/v1/dataset | jq
```

Response:

```sh
{
  "result": {
    "datasets": {
      "tpool/ds1": {
        "name": "tpool/ds1",
        "type": "FILESYSTEM",
        "pool": "tpool",
        "createtxg": "49154",
        "properties": {
          "available": {
            "value": "84.6M",
            "source": {
              "type": "NONE",
              "data": "-"
            }
          },
          "mountpoint": {
            "value": "/tpool/ds1",
            "source": {
              "type": "DEFAULT",
              "data": "-"
            }
          },
          "referenced": {
            "value": "30.6K",
            "source": {
              "type": "NONE",
              "data": "-"
            }
          },
          "used": {
            "value": "30.6K",
            "source": {
              "type": "NONE",
              "data": "-"
            }
          }
        }
      }
    }
  }
}
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
- Run linter, go fmt
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
