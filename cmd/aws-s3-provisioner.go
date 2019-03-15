
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
	"flag"
	"fmt"
	"os"
	"syscall"

	"github.com/aws/aws-sdk-go/aws"
	// s3client "github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/awserr"
	// "github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	awsuser "github.com/aws/aws-sdk-go/service/iam"
	// "github.com/aws/aws-sdk-go-v2/service/iam"

	"github.com/golang/glog"
	"github.com/yard-turkey/lib-bucket-provisioner/pkg/apis/objectbucket.io/v1alpha1"
	libapi "github.com/yard-turkey/lib-bucket-provisioner/pkg/api"
	"github.com/yard-turkey/lib-bucket-provisioner/pkg/api/provisioner"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	//"github.com/crossplane/pkg/controller/storage/bucket"
)

const (
	provisionerName	 = "aws-s3.io/bucket"
	awsAccessKeyName = "AWS_ACCESS_KEY_ID"
	awsSecretKeyName = "AWS_SECRET_ACCESS_KEY"
	s3Host = "s3"
	s3Domain = ".amazonaws.com"
)

type awsS3Provisioner struct {
	bucketName  string
	service	    *s3.S3
	clientset   *kubernetes.Clientset
}

func NewAwsS3Provisioner(cfg rest.Config, s3Provisioner awsS3Provisioner) *libapi.ProvisionerController {

	opts := &libapi.ProvisionerOptions{}

	return libapi.NewProvisioner(&cfg, provisionerName, s3Provisioner, opts)
}

//var _ provisioner.Provisioner = &awsS3Provisioner{}

// return value of string pointer or ""
func StringValue(v *string) string {
	if v != nil {
		return *v
	}
	return ""
}

func createIAM(sess *session.Session) (string, string, error){
	//Create an iam user to pass back as bucket creds???
	myuser := "needtogeneratename"

	iamsvc := awsuser.New(sess)
	uresult, uerr := iamsvc.CreateUser(&awsuser.CreateUserInput{
		UserName: &myuser,
	})
	if uerr != nil {
	  //print error message
	}
	// print out successful result??
	glog.Infof("Successfully created iam user %v", uresult)

	aresult, aerr := iamsvc.CreateAccessKey(&awsuser.CreateAccessKeyInput{
		UserName: &myuser,
	})
	if aerr != nil {
		//print error message
	}
	// print out successful result??
	glog.Infof("Successfully created iam user %v", aresult)
	myaccesskey := StringValue(aresult.AccessKey.AccessKeyId)
	glog.Infof("accessKey = %s", myaccesskey)
	mysecretaccesskey := StringValue(aresult.AccessKey.SecretAccessKey)
	glog.Infof("accessKey = %s", mysecretaccesskey)

	return myaccesskey, mysecretaccesskey, nil

}

// Provision creates a storage asset and returns a PV object representing it.
func (p awsS3Provisioner) Provision(options *provisioner.BucketOptions) (*v1alpha1.Connection, error) {

	// Create a new session and service for aws.Config
	//sess := session.Must(session.NewSession())
	//svc := s3.New(sess)
	sess, _ := session.NewSession(&aws.Config{
		Region: aws.String("us-west-1")},
		// Credentials: credentials.NewStaticCredentials(os.Getenv)
	)
	svc := s3.New(sess)

	//TODO - maybe use private bucket creds in future
	//bucketAccessId, bucketSecretKey, err := createIAM(sess)

	//Create our Bucket
	// where do I get the name from? How do I add in the bucket user to this?
	bucketinput := &s3.CreateBucketInput{
		Bucket: &options.BucketName,
	}

	_, err := svc.CreateBucket(bucketinput)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case s3.ErrCodeBucketAlreadyExists:
				return nil, fmt.Errorf("Bucket %s already exists", p.bucketName)
			case s3.ErrCodeBucketAlreadyOwnedByYou:
				return nil, fmt.Errorf("Bucket %s already owned by you", p.bucketName)
			default:
				return nil, fmt.Errorf("Bucket %s could not be created - %v", p.bucketName, err.Error())
			}
		} else {
			return nil, fmt.Errorf("Bucket %s could not be created - %v", p.bucketName, err.Error())
		}
		return nil, fmt.Errorf("Bucket %s could not be created - %v", p.bucketName, err.Error())
	}

	//build our connectionInfo
	connectionInfo := &v1alpha1.Connection{
		&v1alpha1.Endpoint{
			BucketHost: s3Host,
			BucketPort: 443,
			BucketName: options.BucketName,
			Region:     "us-west-1",
		},
		&v1alpha1.Authentication{
			&v1alpha1.AccessKeys{
				AccessKeyId:     os.Getenv(v1alpha1.AwsKeyField),
				SecretAccessKey: os.Getenv(v1alpha1.AwsSecretField),
			},
		},
	}
	return connectionInfo, nil

}

// Delete OBC??
func (p awsS3Provisioner) Delete(ob *v1alpha1.ObjectBucket) error {
	//TODO
	return nil
}

func main() {
	glog.Infof("AWS S3 Provisioner - main")
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

	s3Prov := awsS3Provisioner{}
	s3Prov.clientset = clientset


	// Create the provisioner: it implements the Provisioner interface expected by
	// the lib
	S3ProvisionerController := NewAwsS3Provisioner(*config, s3Prov)
	S3ProvisionerController.Run()

	//S3ProvisionerController := provisioner.Provisioner(NewAwsS3Provisioner(*config, awsS3Provisioner))
	//// Start the provision controller which will dynamically provision hostPath
	//// PVs
	// pc := controller.NewProvisionController(clientset, provisionerName, hostPathProvisioner, serverVersion.GitVersion)
	//pc.Run(wait.NeverStop)
}
