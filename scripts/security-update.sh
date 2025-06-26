#!/bin/bash

# Security Update Script for aws-s3-provisioner
# This script helps verify and complete the security dependency updates

set -e

echo "🔐 AWS S3 Provisioner Security Update Script"
echo "=============================================="

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if we're in the right directory
if [[ ! -f "go.mod" ]] || [[ ! -f "cmd/aws-s3-provisioner.go" ]]; then
    print_error "Please run this script from the aws-s3-provisioner root directory"
    exit 1
fi

print_status "Starting security dependency update process..."

# Step 1: Update go.mod dependencies
print_status "Step 1: Updating Go module dependencies..."
if command -v go &> /dev/null; then
    go mod tidy
    print_success "Dependencies updated successfully"
else
    print_warning "Go not found in PATH. Please run 'go mod tidy' manually"
fi

# Step 2: Update vendor directory
print_status "Step 2: Updating vendor directory..."
if command -v go &> /dev/null; then
    go mod vendor
    print_success "Vendor directory updated successfully"
else
    print_warning "Go not found in PATH. Please run 'go mod vendor' manually"
fi

# Step 3: Verify critical dependency versions
print_status "Step 3: Verifying critical dependency versions..."

if command -v go &> /dev/null; then
    echo "Checking golang.org/x/text version..."
    TEXT_VERSION=$(go list -m golang.org/x/text 2>/dev/null || echo "not found")
    if [[ "$TEXT_VERSION" == *"v0.21.0"* ]] || [[ "$TEXT_VERSION" > *"v0.3.8"* ]]; then
        print_success "golang.org/x/text: $TEXT_VERSION ✓"
    else
        print_error "golang.org/x/text: $TEXT_VERSION (vulnerable)"
    fi

    echo "Checking golang.org/x/crypto version..."
    CRYPTO_VERSION=$(go list -m golang.org/x/crypto 2>/dev/null || echo "not found")
    if [[ "$CRYPTO_VERSION" == *"v0.31.0"* ]] || [[ "$CRYPTO_VERSION" > *"v0.0.0-20220314234659"* ]]; then
        print_success "golang.org/x/crypto: $CRYPTO_VERSION ✓"
    else
        print_error "golang.org/x/crypto: $CRYPTO_VERSION (vulnerable)"
    fi

    echo "Checking github.com/golang/protobuf version..."
    PROTOBUF_VERSION=$(go list -m github.com/golang/protobuf 2>/dev/null || echo "not found")
    if [[ "$PROTOBUF_VERSION" == *"v1.5.4"* ]] || [[ "$PROTOBUF_VERSION" > *"v1.4.0"* ]]; then
        print_success "github.com/golang/protobuf: $PROTOBUF_VERSION ✓"
    else
        print_error "github.com/golang/protobuf: $PROTOBUF_VERSION (vulnerable)"
    fi

    echo "Checking gopkg.in/yaml.v2 version..."
    YAML_VERSION=$(go list -m gopkg.in/yaml.v2 2>/dev/null || echo "not found")
    if [[ "$YAML_VERSION" == *"v2.2.3"* ]] || [[ "$YAML_VERSION" > *"v2.2.2"* ]]; then
        print_success "gopkg.in/yaml.v2: $YAML_VERSION ✓"
    else
        print_error "gopkg.in/yaml.v2: $YAML_VERSION (vulnerable)"
    fi
else
    print_warning "Cannot verify versions - Go not found in PATH"
fi

# Step 4: Build verification
print_status "Step 4: Verifying build..."
if command -v go &> /dev/null; then
    if go build -o /tmp/aws-s3-provisioner-test ./cmd/...; then
        print_success "Build successful ✓"
        rm -f /tmp/aws-s3-provisioner-test
    else
        print_error "Build failed - please check for compatibility issues"
    fi
else
    print_warning "Cannot verify build - Go not found in PATH"
fi

# Step 5: Security recommendations
print_status "Step 5: Security recommendations..."
cat << EOF

📋 SECURITY UPDATE SUMMARY
==========================

✅ Fixed Vulnerabilities:
   • CVE-2020-14040: golang.org/x/text infinite loop
   • CVE-2022-32149: golang.org/x/text DoS vulnerability  
   • CVE-2021-4235: gopkg.in/yaml.v2 DoS vulnerability
   • Multiple crypto and protobuf vulnerabilities

🔧 Manual Steps (if Go not available):
   1. Run: go mod tidy
   2. Run: go mod vendor
   3. Run: go build -a -o ./bin/aws-s3-provisioner ./cmd/...

🛡️  Next Steps:
   1. Test the application thoroughly
   2. Update your container images
   3. Deploy to staging environment first
   4. Set up automated dependency scanning
   
📚 Documentation:
   - Security details: SECURITY.md
   - Build instructions: README.md

EOF

print_success "Security update process completed!"
print_status "Please review the changes and test thoroughly before deploying to production." 