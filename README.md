# aws-s3-provisioner

### Build and Push the image

1. Build the provisioner binary.
```
 # go build -a -o ./bin/aws-s3-provisioner  ./cmd/...
```

2. Login to docker and quay.io.
```
 # docker login
 # docker login quay.io
```

3. Build the image and push it to quay.io.
```
 # docker build . -t quay.io/<your_quay_account>/aws-s3-provisioner:v1.0.0
 # docker push quay.io/<your_quay_account>/aws-s3-provisioner:v1.0.0
```

i.e.

```
 # docker build . -t quay.io/screeley44/aws-s3-provisioner:v1.0.0
 # docker push quay.io/screeley44/aws-s3-provisioner:v1.0.0
```

### Using the S3 Provisioner to Test the Library

See [bucket library testing](https://github.com/kube-object-storage/lib-bucket-provisioner/tree/master/hack#library-testing).

## Security

⚠️ **Security Alert Resolved**: This project has been updated to address multiple security vulnerabilities, including:

- **CVE-2020-14040 & CVE-2022-32149**: golang.org/x/text vulnerabilities (infinite loop and DoS)
- **CVE-2021-4235**: YAML DoS vulnerability
- **Multiple crypto and protobuf vulnerabilities**

### Security Update Process

To apply security updates and verify the fixes:

```bash
# Run the automated security update script
./scripts/security-update.sh

# Or manually update dependencies
go mod tidy
go mod vendor
go build -a -o ./bin/aws-s3-provisioner ./cmd/...
```

### Quick Security Check
```bash
# Update dependencies to latest secure versions
go mod tidy && go mod vendor

# Verify specific vulnerable packages are updated
go list -m golang.org/x/text golang.org/x/crypto gopkg.in/yaml.v2

# Check for any remaining vulnerable dependencies  
go list -m -u all | grep -E "(golang.org/x/text|golang.org/x/crypto)"
```

For detailed security information, please see [SECURITY.md](SECURITY.md).

### Dependencies Updated
- `golang.org/x/text`: v0.3.0 → v0.21.0
- `golang.org/x/crypto`: v0.0.0-20190313... → v0.31.0  
- `golang.org/x/sys`: v0.0.0-20190215... → v0.28.0
- `github.com/golang/protobuf`: v1.3.0 → v1.5.4
- `gopkg.in/yaml.v2`: v2.2.2 → v2.2.3

