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
[GIN-debug] GET    /api/v1/dataset           --> github.com/stratastor/rodent/pkg/zfs/api.(*DatasetHandler).listDatasets-fm (3 handlers)       [GIN-debug] DELETE /api/v1/dataset           --> github.com/stratastor/rodent/pkg/zfs/api.(*DatasetHandler).destroyDataset-fm (4 handlers)
[GIN-debug] POST   /api/v1/dataset/rename    --> github.com/stratastor/rodent/pkg/zfs/api.(*DatasetHandler).renameDataset-fm (4 handlers)
[GIN-debug] POST   /api/v1/dataset/diff      --> github.com/stratastor/rodent/pkg/zfs/api.(*DatasetHandler).diffDataset-fm (4 handlers)
[GIN-debug] GET    /api/v1/dataset/properties --> github.com/stratastor/rodent/pkg/zfs/api.(*DatasetHandler).listProperties-fm (4 handlers)
[GIN-debug] GET    /api/v1/dataset/property  --> github.com/stratastor/rodent/pkg/zfs/api.(*DatasetHandler).getProperty-fm (5 handlers)
[GIN-debug] PUT    /api/v1/dataset/property  --> github.com/stratastor/rodent/pkg/zfs/api.(*DatasetHandler).setProperty-fm (5 handlers)
[GIN-debug] PUT    /api/v1/dataset/property/inherit --> github.com/stratastor/rodent/pkg/zfs/api.(*DatasetHandler).inheritProperty-fm (5 handle
rs)                                                                                                                                            [GIN-debug] GET    /api/v1/dataset/filesystems --> github.com/stratastor/rodent/pkg/zfs/api.(*DatasetHandler).listFilesystems-fm (3 handlers)
[GIN-debug] POST   /api/v1/dataset/filesystem --> github.com/stratastor/rodent/pkg/zfs/api.(*DatasetHandler).createFilesystem-fm (5 handlers)
[GIN-debug] POST   /api/v1/dataset/filesystem/mount --> github.com/stratastor/rodent/pkg/zfs/api.(*DatasetHandler).mountDataset-fm (5 handlers)
[GIN-debug] POST   /api/v1/dataset/filesystem/unmount --> github.com/stratastor/rodent/pkg/zfs/api.(*DatasetHandler).unmountDataset-fm (4 handl
ers)                                                                                                                                           [GIN-debug] GET    /api/v1/dataset/volumes   --> github.com/stratastor/rodent/pkg/zfs/api.(*DatasetHandler).listVolumes-fm (3 handlers)
[GIN-debug] POST   /api/v1/dataset/volume    --> github.com/stratastor/rodent/pkg/zfs/api.(*DatasetHandler).createVolume-fm (6 handlers)
[GIN-debug] GET    /api/v1/dataset/snapshots --> github.com/stratastor/rodent/pkg/zfs/api.(*DatasetHandler).listSnapshots-fm (3 handlers)
[GIN-debug] POST   /api/v1/dataset/snapshot  --> github.com/stratastor/rodent/pkg/zfs/api.(*DatasetHandler).createSnapshot-fm (5 handlers)
[GIN-debug] POST   /api/v1/dataset/snapshot/rollback --> github.com/stratastor/rodent/pkg/zfs/api.(*DatasetHandler).rollbackSnapshot-fm (4 hand
lers)
[GIN-debug] POST   /api/v1/dataset/clone     --> github.com/stratastor/rodent/pkg/zfs/api.(*DatasetHandler).createClone-fm (6 handlers)
[GIN-debug] POST   /api/v1/dataset/clone/promote --> github.com/stratastor/rodent/pkg/zfs/api.(*DatasetHandler).promoteClone-fm (4 handlers)   [GIN-debug] GET    /api/v1/dataset/bookmarks --> github.com/stratastor/rodent/pkg/zfs/api.(*DatasetHandler).listBookmarks-fm (3 handlers)
[GIN-debug] POST   /api/v1/dataset/bookmark  --> github.com/stratastor/rodent/pkg/zfs/api.(*DatasetHandler).createBookmark-fm (4 handlers)     [GIN-debug] GET    /api/v1/dataset/permissions --> github.com/stratastor/rodent/pkg/zfs/api.(*DatasetHandler).listPermissions-fm (4 handlers)
[GIN-debug] POST   /api/v1/dataset/permissions --> github.com/stratastor/rodent/pkg/zfs/api.(*DatasetHandler).allowPermissions-fm (5 handlers)
[GIN-debug] DELETE /api/v1/dataset/permissions --> github.com/stratastor/rodent/pkg/zfs/api.(*DatasetHandler).unallowPermissions-fm (5 handlers
)
[GIN-debug] POST   /api/v1/dataset/share     --> github.com/stratastor/rodent/pkg/zfs/api.(*DatasetHandler).shareDataset-fm (3 handlers)
[GIN-debug] DELETE /api/v1/dataset/share     --> github.com/stratastor/rodent/pkg/zfs/api.(*DatasetHandler).unshareDataset-fm (3 handlers)     [GIN-debug] POST   /api/v1/dataset/transfer/send --> github.com/stratastor/rodent/pkg/zfs/api.(*DatasetHandler).sendDataset-fm (3 handlers)
[GIN-debug] GET    /api/v1/dataset/transfer/resume-token --> github.com/stratastor/rodent/pkg/zfs/api.(*DatasetHandler).getResumeToken-fm (4 ha
ndlers)


[GIN-debug] POST   /api/v1/pools             --> github.com/stratastor/rodent/pkg/zfs/api.(*PoolHandler).createPool-fm (7 handlers)
[GIN-debug] GET    /api/v1/pools             --> github.com/stratastor/rodent/pkg/zfs/api.(*PoolHandler).listPools-fm (3 handlers)
[GIN-debug] DELETE /api/v1/pools/:name       --> github.com/stratastor/rodent/pkg/zfs/api.(*PoolHandler).destroyPool-fm (4 handlers)
[GIN-debug] POST   /api/v1/pools/import      --> github.com/stratastor/rodent/pkg/zfs/api.(*PoolHandler).importPool-fm (4 handlers)
[GIN-debug] POST   /api/v1/pools/:name/export --> github.com/stratastor/rodent/pkg/zfs/api.(*PoolHandler).exportPool-fm (4 handlers)
[GIN-debug] GET    /api/v1/pools/:name/status --> github.com/stratastor/rodent/pkg/zfs/api.(*PoolHandler).getPoolStatus-fm (4 handlers)
[GIN-debug] GET    /api/v1/pools/:name/properties/:property --> github.com/stratastor/rodent/pkg/zfs/api.(*PoolHandler).getProperty-fm (5 handl
ers)
[GIN-debug] PUT    /api/v1/pools/:name/properties/:property --> github.com/stratastor/rodent/pkg/zfs/api.(*PoolHandler).setProperty-fm (6 handl
ers)
[GIN-debug] POST   /api/v1/pools/:name/scrub --> github.com/stratastor/rodent/pkg/zfs/api.(*PoolHandler).scrubPool-fm (4 handlers)
[GIN-debug] POST   /api/v1/pools/:name/resilver --> github.com/stratastor/rodent/pkg/zfs/api.(*PoolHandler).resilverPool-fm (4 handlers)
[GIN-debug] POST   /api/v1/pools/:name/devices/attach --> github.com/stratastor/rodent/pkg/zfs/api.(*PoolHandler).attachDevice-fm (4 handlers)
[GIN-debug] POST   /api/v1/pools/:name/devices/detach --> github.com/stratastor/rodent/pkg/zfs/api.(*PoolHandler).detachDevice-fm (4 handlers)
[GIN-debug] POST   /api/v1/pools/:name/devices/replace --> github.com/stratastor/rodent/pkg/zfs/api.(*PoolHandler).replaceDevice-fm (4 handlers
)
```

[API test cases](./pkg/zfs/api/dataset_test.go) provides reference usage but perhaps `curl` commands might illustrate it cleaner.

Assuming zfs pool `tpool` is already created, and available, try the following:

1. Run rodent server

```sh
ubuntu@staging:~/rodent$ sudo go run main.go serve
```

2. Create the request payload with the following data:

```sh
ubuntu@staging:~/curl-test$ cat create.json
{
    "name": "tank/dummyds2",
    "dry_run": false,
    "verbose": true,
    "parsable": true
}
```

```sh
ubuntu@staging:~/curl-test$ cat list.json
{
        "name": "tpool/ds1",
        "type": "all",
        "recursive": true
}
```

3. Make the `curl` requests

```sh
ubuntu@staging:~/curl-test$ curl -s -S --json @create.json -X POST http://localhost:8042/api/v1/dataset/filesystem
```

```sh
ubuntu@staging:~/curl-test$ curl -s -S --json @list.json -X GET http://localhost:8042/api/v1/dataset | jq
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
