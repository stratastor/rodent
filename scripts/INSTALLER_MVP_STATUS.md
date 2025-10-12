# Rodent Installer MVP - Status

## Objectives

Build a seamless installer for Rodent supporting Ubuntu 24+ (amd64/arm64) with interactive and automated modes, smart dependency management, production-safe upgrades, and telemetry collection.

**Distribution:** One-line install via `curl -fsSL https://utils.strata.host/install.sh | sudo bash`

---

## ✅ MVP Complete

### Core Components

- **Build system:** Go 1.25.2 multi-arch builds with version injection ([build.sh](../scripts/build/build.sh))
- **Bootstrap installer:** Pre-flight checks, downloads full installer ([bootstrap.sh](../scripts/install/bootstrap.sh))
- **Full installer:** OS checks, dependency installation, user setup, telemetry ([install-rodent.sh](../scripts/install/install-rodent.sh))
- **Setup script:** User/directory creation, sudoers, config management ([setup_rodent_user.sh](../scripts/setup_rodent_user.sh))

### Dependencies Managed

- ZFS 2.3+ (Zabbly repository)
- Docker (official repository)
- Samba + Kerberos (non-interactive configuration)
- System utilities (acl, attr, smartmontools, jq, ripgrep)

### CI/CD Pipeline

- **GitHub Actions:** Multi-arch builds, R2 uploads, GitHub releases ([build-and-release.yml](../.github/workflows/build-and-release.yml))
- **R2 Distribution:** Binaries at pkg.strata.host, scripts at utils.strata.host
- **Dev workflow:** Quick upload script for testing ([dev-upload.sh](../scripts/build/dev-upload.sh))

### Testing & Validation

- ✅ End-to-end installation on Ubuntu 24.04
- ✅ ZFS version detection (2.3.4 verified)
- ✅ Sudoers file creation and permissions
- ✅ Config file management (preserves existing)
- ✅ Upgrade scenario (config preservation, binary backup)
- ✅ Telemetry collection (install_type: fresh/reinstall/upgrade)
- ✅ GitHub Actions pipeline (binaries + scripts uploaded to R2)

---

## 📋 Deferred Items

### Short-term

- Log rotation configuration (`/etc/logrotate.d/rodent`)
- Service start validation and common error handling

### Phase 2 - Enterprise Features

- APT repository setup
- Semantic versioning and upgrade path validation
- Rollback functionality
- Health checks and pre-flight validation
- Multi-distribution support (Ubuntu 22.04, Debian)
- Air-gapped installation
- Binary signature verification (GPG)
- Telemetry dashboard

---

## 📝 Key Design Decisions

- **R2 over S3:** Cost efficiency and Cloudflare CDN integration
- **Binary distribution over APT:** Faster iteration for MVP
- **Zabbly ZFS repo:** Ubuntu default is 2.2, need 2.3+
- **Non-interactive Kerberos:** debconf pre-configuration for automation
- **No caching during dev:** Scripts use `cache-control: no-cache` for rapid testing
- **rodent.sample.yml naming:** Gitignored pattern, no manual git add needed
- **Bucket-specific R2 testing:** Token lacks ListBuckets permission
- **Skip 'latest' copy when VERSION=latest:** Avoid redundant S3 operations

---

## 🔗 Resources

- **Installation URL:** <https://utils.strata.host/install.sh>
- **Binaries:** <https://pkg.strata.host/rodent/latest/>
- **Documentation:** [INSTALLATION.md](../docs/INSTALLATION.md)

---

**Status:** MVP Complete
**Last Updated:** 2025-10-13
