
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
	libbkt "github.com/yard-turkey/lib-bucket-provisioner/pkg/provisioner"
	apibkt "github.com/yard-turkey/lib-bucket-provisioner/pkg/provisioner/api"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	restclient "k8s.io/client-go/rest"
	storage "k8s.io/api/storage/v1"

	//"github.com/crossplane/pkg/controller/storage/bucket"
	//"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"github.com/aws/aws-sdk-go/aws/credentials"
)

const (
	provisionerName	 = "aws-s3.io/bucket"
	s3Host = "s3"
	s3Domain = ".amazonaws.com"
)

var (
	kubeconfig string
	masterURL  string
)

type awsS3Provisioner struct {
	bucketName  string
	service	    *s3.S3
	clientset   *kubernetes.Clientset
}

func NewAwsS3Provisioner(cfg *restclient.Config, s3Provisioner awsS3Provisioner) *libbkt.ProvisionerController {

	opts := &libbkt.ProvisionerOptions{}
	return libbkt.NewProvisioner(cfg, provisionerName, s3Provisioner, opts)
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

//TODO don't really need this?
func (p awsS3Provisioner)getObjectBucketClaimClass(obc *v1alpha1.ObjectBucketClaim) string {
	return obc.Spec.StorageClassName
}


// GetClassForVolume locates storage class by persistent volume
func (p awsS3Provisioner) getClassForBucketClaim(obc *v1alpha1.ObjectBucketClaim) (*storage.StorageClass, error) {
	if p.clientset == nil {
		return nil, fmt.Errorf("Cannot get kube client")
	}
	className := obc.Spec.StorageClassName
	if className == "" {
		// keep trying to find credentials or storageclass?
		glog.Infof("OBC has no storageclass - should we now look at env?")
	}

	//not sure how to get the storage class
	if className != "" {
		class, err := p.clientset.StorageV1().StorageClasses().Get(className, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}

		return class, nil
	}

	return nil, nil
}

// Provision creates a storage asset and returns a PV object representing it.
func (p awsS3Provisioner) Provision(options *apibkt.BucketOptions) (*v1alpha1.Connection, error) {

	// create general session object
	sess := &session.Session{}

	//TODO: move to constants
	// set some defaults
	region := "us-west-1"
	ns := "default"
	secretName := "s3-bucket-owner"
	accessId := ""
	accessSecret := ""
	glog.Infof("In Provision - default region is %s and default ns is %s", region, ns)

	// Get the storageclass here so we can get credentials?
	sc, _ := p.getClassForBucketClaim(options.ObjectBucketClaim)

	if sc != nil && sc.Parameters["secretName"] != "" {
		glog.Infof("  StorageClass does exist in claim - %s", sc.Name)

		//get our region, ns and secret stuff
		if sc.Parameters["region"] != "" {
			region = sc.Parameters["region"]
		}
		if sc.Parameters["secretNamespace"] != "" {
			ns = sc.Parameters["secretNamespace"]
		}
		if sc.Parameters["secretName"] != "" {
			secretName = sc.Parameters["secretName"]
		}

		//now get our secret ref
		secretkeys, err := p.clientset.CoreV1().Secrets(ns).Get(secretName, metav1.GetOptions{})
		if err != nil {
			//log something
		}
		accessId := string(secretkeys.Data[v1alpha1.AwsKeyField])
		accessSecret := string(secretkeys.Data[v1alpha1.AwsSecretField])
		glog.Infof("Access Keys are empty - %s %s", accessId, accessSecret)

		if len(accessId) > 0 || len(accessSecret) >0 {
			// use our SC to create our session
			sess, _ = session.NewSession(&aws.Config{
				Region:      aws.String(region),
				Credentials: credentials.NewStaticCredentials(accessId, accessSecret, "TOKEN"),
			})
		} else {
			// fall back to default implementation
			glog.Infof("  Could Not Find accessKey and Secret - %s", options.ObjectBucketClaim.Name)
			sess, _ = session.NewSession(&aws.Config{
				Region: aws.String(region)},
			)
		}
	} else {
		// this should follow normal AWS credential chain
		// meaning it will look in default config for aws cli (aws configure)
		// or it will look for env vars
		glog.Infof("  No StorageClass in OBC - %s", options.ObjectBucketClaim.Name)
		glog.Infof("  Access Keys are empty as expected - [%s] [%s]", accessId, accessSecret)
		sess, _ = session.NewSession(&aws.Config{
			Region: aws.String(region)},
			// Credentials: credentials.NewStaticCredentials(os.Getenv)
		)
	}

	// Create a new session and service for aws.Config
	//sess := session.Must(session.NewSession())
	//svc := s3.New(sess)

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
			Region:     region,
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

// --kubeconfig and --master are set in the controller-runtime's config
// package's init(). Set global kubeconfig and masterURL variables depending
// on flag values or env variables. Also sets `alsologtostderr`.
func handle_flags() {

	flag.Parse()
	flag.Set("logtostderr", "true")

	flag.VisitAll(func(f *flag.Flag) {
		if f.Name == "kubeconfig" {
			kubeconfig = flag.Lookup(f.Name).Value.String()
			if kubeconfig == "" {
				kubeconfig = os.Getenv("KUBECONFIG")
			}
			return
		}
		if f.Name == "master" {
			masterURL = flag.Lookup(f.Name).Value.String()
			if masterURL == "" {
				masterURL = os.Getenv("MASTER")
			}
			return
		}
	})
}

// create k8s config and client for the runtime-controller. 
// Note: panics on errors.
func createConfigAndClientOrDie(masterurl, kubeconfig string) (*restclient.Config, *kubernetes.Clientset) {
	config, err := clientcmd.BuildConfigFromFlags(masterurl, kubeconfig)
	if err != nil {
		glog.Fatalf("Failed to create config: %v", err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		glog.Fatalf("Failed to create client: %v", err)
	}
	return config, clientset
}

func main() {
	syscall.Umask(0)

	handle_flags()

	glog.Infof("AWS S3 Provisioner - main")
	glog.Infof("flags: kubeconfig=%q; masterURL=%q", kubeconfig, masterURL)

	config, clientset := createConfigAndClientOrDie(masterURL, kubeconfig)

	s3Prov := awsS3Provisioner{}
	s3Prov.clientset = clientset

	// Create and run the s3 provisioner controller.
	// It implements the Provisioner interface expected by the bucket
	// provisioning lib.
	S3ProvisionerController := NewAwsS3Provisioner(config, s3Prov)
	S3ProvisionerController.Run()
}
