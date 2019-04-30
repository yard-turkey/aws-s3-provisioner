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