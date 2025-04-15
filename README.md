# Rodent [WIP]

[![Go Report Card](https://goreportcard.com/badge/gojp/goreportcard)](https://goreportcard.com/report/github.com/stratastor/rodent) [![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://github.com/stratastor/rodent/blob/master/LICENSE.txt)

Rodent is a StrataSTOR node agent for ZFS operations(primarily).

## Overview

Rodent is a comprehensive agent designed to manage ZFS-based storage systems. It provides a robust set of features for modern Network Attached Storage (NAS) environments:

### Key Features

- **ZFS Pool Management**: Create, import, export, and monitor ZFS pools
- **Dataset Operations**: Manage filesystems, volumes, and snapshots
- **Data Sharing**: Support for SMB/CIFS, NFS, and iSCSI protocols
- **User Management**: Integrated access control and permission management
- **Data Protection**: Snapshot scheduling, replication, and backup workflows
- **Performance Monitoring**: Real-time metrics and health reporting
- **REST API**: Programmatic access to all functionality

Rodent serves as the operational layer between the ZFS subsystem and higher-level storage management interfaces, providing a unified approach to storage administration.

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
