# Security Policy

## CVE-2021-4235 - YAML Denial of Service Vulnerability

### Overview
This project was affected by CVE-2021-4235 (GHSA-r88r-gmrh-7j83), a denial of service vulnerability in the `gopkg.in/yaml.v2` library.

### Vulnerability Details
- **CVE ID**: CVE-2021-4235
- **GHSA ID**: GHSA-r88r-gmrh-7j83
- **Severity**: Moderate (CVSS 5.5)
- **Description**: Due to unbounded alias chasing, a maliciously crafted YAML file can cause the system to consume significant system resources.

### Affected Versions
- `gopkg.in/yaml.v2` versions <= 2.2.2

### Fixed Versions
- `gopkg.in/yaml.v2` versions >= 2.2.3

### Mitigation Applied

#### 1. Dependency Update
- Updated `gopkg.in/yaml.v2` from v2.2.2 to v2.2.3 in go.mod
- This fixes the unbounded alias chasing vulnerability

#### 2. Input Validation
- Added `validateYAMLInput()` function in `cmd/util.go`
- Implements size limits (1MB max) for YAML content
- Detects suspicious alias patterns that could indicate malicious input
- Limits alias depth to prevent resource exhaustion

#### 3. Attack Surface Analysis
The vulnerability could potentially be exploited through:
- Malicious ObjectBucketClaim YAML manifests
- Corrupted StorageClass configurations
- Malicious data processed through Kubernetes APIs

### Security Best Practices

#### For Developers
1. **Keep Dependencies Updated**: Regularly update all dependencies, especially security-critical ones
2. **Input Validation**: Always validate external input before processing
3. **Resource Limits**: Implement reasonable resource limits for parsing operations
4. **Security Scanning**: Use tools like `go list -m -u all` to check for vulnerable dependencies

#### For Operators
1. **Access Control**: Limit who can create/modify ObjectBucketClaims and StorageClasses
2. **Resource Monitoring**: Monitor resource usage for unusual spikes during YAML processing
3. **Regular Updates**: Keep the provisioner updated with latest security patches

#### Commands to Check for Vulnerabilities
```bash
# Check for vulnerable dependencies
go list -m -u all | grep yaml

# Run security audit (if using Go 1.18+)
go list -json -deps | nancy sleuth

# Update dependencies
go mod tidy && go mod vendor
```

### Reporting Security Issues
If you discover a security vulnerability, please report it privately by:
1. Creating a private GitHub issue
2. Emailing the maintainers directly
3. Following responsible disclosure practices

### Security Contact
For security-related inquiries, please contact the project maintainers.

---

**Last Updated**: $(date +%Y-%m-%d)
**Next Review**: $(date -d '+6 months' +%Y-%m-%d) 