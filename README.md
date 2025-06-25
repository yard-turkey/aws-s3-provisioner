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

This project has been updated to address security vulnerabilities, including CVE-2021-4235 (YAML DoS vulnerability). 

For detailed security information, please see [SECURITY.md](SECURITY.md).

### Quick Security Check
```bash
# Update dependencies to latest secure versions
go mod tidy && go mod vendor

# Check for vulnerable dependencies  
go list -m -u all | grep yaml
```

