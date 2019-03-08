
/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path"
	"syscall"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"

	"github.com/golang/glog"
	v1alpha1 "github.com/yard-turkey/lib-bucket-provisioner/pkg/apis/objectbucket.io/v1alpha1"
	provisioner "github.com/yard-turkey/lib-bucket-provisioner/pkg/api/provisioner"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

)

const (
	provisionerName = "aws-s3.io/bucket"
	awsKey = "AWS_ACCESS_KEY_ID"
	awsSecret = "AWS_SECRET_ACCESS_KEY"
)

type awss3Provisioner struct {
	// The directory to create PV-backing directories in
	endpointUrl string
	bucketName  string
	accessKey   string
	secretKey   string
}

// NewAwsS3Provisioner creates a new aws s3 provisioner
func NewAwsS3Provisioner() provisioner.Provisioner {
	bname := os.Getenv("BUCKET_NAME")
	glog.Infof("% is our bucket", bname)
	user := os.Getenv(awsKey)
	glog.Infof("% is our user", user)
	pword := os.Getenv(awsSecret)
	glog.Infof("% is our pword", pword)


	//how do I fill these in?
	return &awss3Provisioner{
		endpointUrl:   "some.url.here",
		bucketName:    bname,
		accessKey:     user,
		secretKey:     pword,
	}

}

var _ provisioner.Provisioner = &awss3Provisioner{}

// Provision creates a storage asset and returns a PV object representing it.
func (p *awss3Provisioner)Provision(options *provisioner.BucketOptions) (*v1alpha1.ObjectBucket, *provisioner.S3AccessKeys, error) {

	//sess := session.Must(session.NewSession())
	//svc := s3.New(sess)

	// Create a S3 client instance from a session
	// - is aws.Config just a default config is that passed in from the library?
	// - where do I get the bucket input - I'm guessing that is passed in from BucketOptions from the lib

	sess, _ := session.NewSession(&aws.Config{
		Region: aws.String("us-west-1")},
	)
	svc := s3.New(sess)

	// where do I get the name from?
	bucketinput := &s3.CreateBucketInput{
		Bucket: aws.String(options.BucketName),
	}

	_, err := svc.CreateBucket(bucketinput)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case s3.ErrCodeBucketAlreadyExists:
				return nil, nil, fmt.Errorf("Bucket %s already exists", p.bucketName)
			case s3.ErrCodeBucketAlreadyOwnedByYou:
				return nil, nil, fmt.Errorf("Bucket %s already owned by you", p.bucketName)
			default:
				return nil, nil, fmt.Errorf("Bucket %s could not be created - %v", p.bucketName, err.Error())
			}
		} else {
			return nil, nil, fmt.Errorf("Bucket %s could not be created - %v", p.bucketName, err.Error())
		}
		return nil, nil, fmt.Errorf("Bucket %s could not be created - %v", p.bucketName, err.Error())
	}

	return nil, nil, nil

}

// Delete OBC??
func (p *awss3Provisioner) Delete(ob *v1alpha1.ObjectBucket) error {
	//TODO
	return nil
}

func main() {
	syscall.Umask(0)

	flag.Parse()
	flag.Set("logtostderr", "true")

	// Create an InClusterConfig and use it to create a client for the controller
	// to use to communicate with Kubernetes
	config, err := rest.InClusterConfig()
	if err != nil {
		glog.Fatalf("Failed to create config: %v", err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		glog.Fatalf("Failed to create client: %v", err)
	}


	// Create the provisioner: it implements the Provisioner interface expected by
	// the lib
	awss3Provisioner := NewAwsS3Provisioner()

	//// Start the provision controller which will dynamically provision hostPath
	//// PVs
	// pc := controller.NewProvisionController(clientset, provisionerName, hostPathProvisioner, serverVersion.GitVersion)
	//pc.Run(wait.NeverStop)
}
