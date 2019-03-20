
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
	"os/signal"
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
		// Yes, w/ exponential backoff
		return nil, fmt.Errorf("StorageClass missing in OBC %q", obc.Name)
	}

	class, err := p.clientset.StorageV1().StorageClasses().Get(className, metav1.GetOptions{})
	// TODO: retry w/ exponential backoff
	if err != nil {
		return nil, err
	}
	return class, nil
}

func awsDefaultSession(region string) (*session.Session, error) {

	glog.Infof("Creating AWS Default Session")
	return session.NewSession(&aws.Config{
			Region: aws.String(region),
			//Credentials: credentials.NewStaticCredentials(os.Getenv),
		})
}

func createBucket(svc *s3.S3, name, region string) (*v1alpha1.Connection, error) {

	bucketinput := &s3.CreateBucketInput{
		Bucket: &name,
	}

	_, err := svc.CreateBucket(bucketinput)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case s3.ErrCodeBucketAlreadyExists:
				return nil, fmt.Errorf("Bucket %q already exists", name)
			case s3.ErrCodeBucketAlreadyOwnedByYou:
				return nil, fmt.Errorf("Bucket %q already owned by you", name)
			}
		}
		return nil, fmt.Errorf("Bucket %q could not be created: %v", name, err)
	}

	glog.Infof("Bucket %s successfully created", name)
	return  &v1alpha1.Connection{
		&v1alpha1.Endpoint{
			BucketHost: s3Host,
			BucketPort: 443,
			BucketName: name,
			Region:     region,
		},
		&v1alpha1.Authentication{
			&v1alpha1.AccessKeys{
				AccessKeyId:     os.Getenv(v1alpha1.AwsKeyField),
				SecretAccessKey: os.Getenv(v1alpha1.AwsSecretField),
			},
		},
	}, nil
}

// Get the secret namespace and name from the passed in map. 
// Empty strings are also returned.
func getSecretName(parms map[string]string) (ns, name string) {

	const (
		scSecretNameKey = "secretName"
		scSecretNSKey   = "secretNamespace"
	)

	ns, _ = parms[scSecretNSKey]
	name, _ = parms[scSecretNameKey]
	return
}

// Get the secret and return its accessKeyId string and secretKey string.
func credsFromSecret(c *kubernetes.Clientset, ns, name string) (string, string, error) {
	secret, err := c.CoreV1().Secrets(ns).Get(name, metav1.GetOptions{})
	if err != nil {
		return "", "", fmt.Errorf("unable to get Secret \"%s/%s\" with error %v", ns, name, err)
	}

	return string(secret.Data[v1alpha1.AwsKeyField]), string(secret.Data[v1alpha1.AwsSecretField]), nil
}


// Provision creates an aws s3 bucket and returns a connection info representing
// the bucket's endpoint and user access credentials.
func (p awsS3Provisioner) Provision(options *apibkt.BucketOptions) (*v1alpha1.Connection, error) {

	const (
		defaultRegion   = "us-west-1"
		scRegionKey     = "region"
	)
	var region string

	// create general session object -- WHY Scott??
	sess := &session.Session{}

	// Get the storageclass here so we can get credentials?
	sc, err := p.getClassForBucketClaim(options.ObjectBucketClaim)
	if err != nil || sc == nil {
		glog.Infof("StorageClass missing in OBC %q", options.ObjectBucketClaim.Name)
		// fall back to using aws config on host node which should
		// follow normal AWS credential chain
		sess, err = awsDefaultSession(defaultRegion)

	} else {
		// get expected sc parameters containing region and secret 
		var ok bool
		region, ok = sc.Parameters[scRegionKey]
		if !ok || region == "" {
			region = defaultRegion
		}
		secretNS, secretName := getSecretName(sc.Parameters)
		if secretNS == "" || secretName == "" {
			glog.Warningf("secret name or namespace are empty in storage class %q", sc.Name)
			sess, err = awsDefaultSession(region)
		} else {
			// get the sc's bucket owner secret
			accessId, accessSecret, err := credsFromSecret(p.clientset, secretNS, secretName)
			if err != nil || len(accessId) == 0 || len(accessSecret) == 0 {
				glog.Warningf("secret \"%s/%s\" in storage class %q for OBC %q is empty.\nUsing default credentials.", secretNS, secretName, sc.Name, options.ObjectBucketClaim.Name)
				sess, err = awsDefaultSession(region)
				if err != nil {
					glog.Errorf("session not being created from awsDefaultSession %s %v", region, err)
				}
				if sess == nil {
					glog.Warningf("session is nil")
				}
			} else {
				// use the OBC's SC to create our session
				glog.Infof("Creating session using static credentials from storageclass secret")
				sess, err = session.NewSession(&aws.Config{
					Region:      aws.String(region),
					Credentials: credentials.NewStaticCredentials(accessId, accessSecret, ""),
				})
			}
		}
	}

	// Create a new service for aws.Config
	svc := s3.New(sess)

	//TODO - maybe use private bucket creds in future
	//bucketAccessId, bucketSecretKey, err := createIAM(sess)

	return createBucket(svc, options.BucketName, region)
}

// Delete OBC??
func (p awsS3Provisioner) Delete(ob *v1alpha1.ObjectBucket) error {
	//TODO
	return nil
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
	defer glog.Flush()
	syscall.Umask(0)

	handleFlags()

	glog.Infof("AWS S3 Provisioner - main")
	glog.Infof("flags: kubeconfig=%q; masterURL=%q", kubeconfig, masterURL)

	config, clientset := createConfigAndClientOrDie(masterURL, kubeconfig)

	stopCh := handleSignals()

	s3Prov := awsS3Provisioner{}
	s3Prov.clientset = clientset

	// Create and run the s3 provisioner controller.
	// It implements the Provisioner interface expected by the bucket
	// provisioning lib.
	S3ProvisionerController := NewAwsS3Provisioner(config, s3Prov)
	glog.Infof("main: running %s provisioner...", provisionerName)
	S3ProvisionerController.Run()

	<-stopCh
	glog.Infof("main: %s provisioner exited.", provisionerName)
}

// --kubeconfig and --master are set in the controller-runtime's config
// package's init(). Set global kubeconfig and masterURL variables depending
// on flag values or env variables. 
// Note: `alsologtostderr` *must* be specified on the command line to see
//   provisioner and bucketlibrary logging. Setting it here does not affect
//   the lib because its init() function has already run.
func handleFlags() {

	if !flag.Parsed() {
		flag.Parse()
	}

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

// Shutdown gracefully on system signals.
func handleSignals() <-chan struct{} {
	sigCh := make(chan os.Signal)
	stopCh := make(chan struct{})
	go func() {
		signal.Notify(sigCh)
		<-sigCh
		close(stopCh)
		os.Exit(1)
	}()
	return stopCh
}
