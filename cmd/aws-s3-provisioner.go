
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
	_ "net/url"
	"os"
	"os/signal"
	"strings"
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
	defaultRegion	= "us-west-1"
	httpPort	= 80
	httpsPort	= 443
	provisionerName	= "aws-s3.io/bucket"
	regionInsert	= "<REGION>"
	s3Hostname	= "s3-"+regionInsert+".amazonaws.com"
)

var (
	kubeconfig string
	masterURL  string
)

type awsS3Provisioner struct {
	region	  string
	// session is the aws session
	session	  *session.Session
	// service is the aws s3 service based on the session
	service	  *s3.S3
	clientset *kubernetes.Clientset
	// access keys for aws acct for the bucket *owner*
	bktOwnerAccessId  string
	bktOwnerSecretKey string
}

func NewAwsS3Provisioner(cfg *restclient.Config, s3Provisioner awsS3Provisioner) *libbkt.ProvisionerController {

	opts := &libbkt.ProvisionerOptions{}
	return libbkt.NewProvisioner(cfg, provisionerName, s3Provisioner, opts)
}

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

// Return the aws default session.
func awsDefaultSession() (*session.Session, error) {

	glog.Infof("Creating AWS *default* session")
	return session.NewSession(&aws.Config{
			Region: aws.String(defaultRegion),
			//Credentials: credentials.NewStaticCredentials(os.Getenv),
		})
}

func (p awsS3Provisioner) createBucket(name string) (*v1alpha1.Connection, error) {

	bucketinput := &s3.CreateBucketInput{
		Bucket: &name,
	}

	_, err := p.service.CreateBucket(bucketinput)
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

	host := strings.Replace(s3Hostname, regionInsert, p.region, 1)
	return  &v1alpha1.Connection{
		&v1alpha1.Endpoint{
			BucketHost: host,
			BucketPort: httpsPort,
			BucketName: name,
			Region:     p.region,
			SSL:	    true,
		},
		&v1alpha1.Authentication{
			&v1alpha1.AccessKeys{
				AccessKeyId:     p.bktOwnerAccessId,
				SecretAccessKey: p.bktOwnerSecretKey,
			},
		},
	}, nil
}

// Get the secret namespace and name from the passed in map. 
// Empty strings are also returned.
func getSecretName(parms map[string]string) (string, string) {

	const (
		scSecretNameKey = "secretName"
		scSecretNSKey   = "secretNamespace"
	)
	return parms[scSecretNSKey], parms[scSecretNameKey]
}

// Get the secret and set the receiver to the accessKeyId and secretKey.
func (p *awsS3Provisioner) credsFromSecret(c *kubernetes.Clientset, ns, name string) error {
	secret, err := c.CoreV1().Secrets(ns).Get(name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("unable to get Secret \"%s/%s\" with error %v", ns, name, err)
	}

	accessKeyId := string(secret.Data[v1alpha1.AwsKeyField])
	secretKey := string(secret.Data[v1alpha1.AwsSecretField])
	if accessKeyId == "" || secretKey == "" {
		return fmt.Errorf("accessId and/or secretKey are blank in secret \"%s/%s\"", secret.Namespace, secret.Name)
	}

	// set receiver fields
	p.bktOwnerAccessId = accessKeyId
	p.bktOwnerSecretKey = secretKey
	return nil
}

// Create an aws session based on the OBC's storage class's secret and region.
// Set in the receiver the session and region used to create the session.
// Note: in error cases it's possible that the returned region is different
//   from the OBC's storage class's region.
func (p *awsS3Provisioner) awsSessionFromOBC(obc *v1alpha1.ObjectBucketClaim) error {

	const scRegionKey = "region"
	var err error

	// helper func var for error returns
	var errDefault = func() error {
		p.region = defaultRegion
		p.session, err = awsDefaultSession()
		return err
	}

	sc, err := p.getClassForBucketClaim(obc)
	if err != nil {
		glog.Errorf("Get failed for storage class %q", obc.Spec.StorageClassName)
		return errDefault()
	}
	
	// get expected sc parameters containing region and secret 
	region, ok := sc.Parameters[scRegionKey]
	if !ok || region == "" {
		region = defaultRegion
	}
	secretNS, secretName := getSecretName(sc.Parameters)
	if secretNS == "" || secretName == "" {
		glog.Warningf("secret name or namespace are empty in storage class %q", sc.Name)
		return errDefault()
	}

	// get the sc's bucket owner secret
	err = p.credsFromSecret(p.clientset, secretNS, secretName)
	if err != nil {
		glog.Warningf("secret \"%s/%s\" in storage class %q for OBC %q is empty.\nUsing default credentials.", secretNS, secretName, sc.Name, obc.Name)
		return errDefault()
	}

	// use the OBC's SC to create our session, set receiver fields
	glog.Infof("Creating aws session using credentials from storage class %s's secret", sc.Name)
	p.region = region
	p.session, err = session.NewSession(&aws.Config{
			Region:      aws.String(region),
			Credentials: credentials.NewStaticCredentials(p.bktOwnerAccessId, p.bktOwnerSecretKey, ""),
	})

	return err
}

// Provision creates an aws s3 bucket and returns a connection info
// representing the bucket's endpoint and user access credentials.
func (p awsS3Provisioner) Provision(options *apibkt.BucketOptions) (*v1alpha1.Connection, error) {

	obc := options.ObjectBucketClaim

	// set the aws session from the obc in the receiver
	err := p.awsSessionFromOBC(obc)
	if err != nil {
		return nil, fmt.Errorf("error creating session from OBC \"%s/%s\": %v", obc.Namespace, obc.Name, err)
	}

	glog.Infof("Creating S3 service for OBC \"%s/%s\"", obc.Namespace, obc.Name)
	p.service = s3.New(p.session)

	//TODO - maybe use private bucket creds in future
	//bucketAccessId, bucketSecretKey, err := createIAM(sess)

	glog.Infof("Creating bucket %q", options.BucketName)
	conn, err := p.createBucket(options.BucketName)
	if err != nil {
		glog.Errorf("error creating bucket %q: %v", options.BucketName, err)
	}
	return conn, err
}

// Delete the bucket references in the passed-in OB.
func (p awsS3Provisioner) Delete(ob *v1alpha1.ObjectBucket) error {

	bktName := ob.Spec.Endpoint.BucketName
	bucketinput := &s3.DeleteBucketInput{
		Bucket: &bktName,
	}
	glog.Infof("Deleting bucket %q via OB %q", bktName, ob.Name)

	_, err := p.service.DeleteBucket(bucketinput)
	return err
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
