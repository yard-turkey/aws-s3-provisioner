# Security Policy

## Recent Security Fixes

### CVE-2020-14040 & CVE-2022-32149 - golang.org/x/text Vulnerabilities

#### Overview
This project was affected by multiple vulnerabilities in the `golang.org/x/text` library, including infinite loop and DoS vulnerabilities.

#### Vulnerability Details
- **CVE ID**: CVE-2020-14040, CVE-2022-32149
- **Severity**: High/Medium
- **Description**: Multiple vulnerabilities including infinite loop in unicode normalization and DoS in language package
- **Affected Versions**: golang.org/x/text versions < 0.3.8
- **Fixed Versions**: golang.org/x/text >= 0.3.8

#### Additional Dependencies Updated
- **golang.org/x/crypto**: Updated from v0.0.0-20190313024323-a1f597ede03a to v0.31.0
- **golang.org/x/sys**: Updated from v0.0.0-20190215142949-d0b11bdaac8a to v0.28.0  
- **github.com/golang/protobuf**: Updated from v1.3.0 to v1.5.4

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
go list -m -u all | grep -E "(golang.org/x/text|golang.org/x/crypto)"

# Run security audit (if using Go 1.18+)
go list -json -deps | nancy sleuth

# Update dependencies
go mod tidy && go mod vendor

# Check specific vulnerable packages
go list -m golang.org/x/text golang.org/x/crypto
```

### Reporting Security Issues
If you discover a security vulnerability, please report it privately by:
1. Creating a private GitHub issue
2. Emailing the maintainers directly
3. Following responsible disclosure practices

### Security Contact
For security-related inquiries, please contact the project maintainers.

---

**Last Updated**: 2024-12-19
**Next Review**: 2025-06-19 