
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
	"encoding/json"
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
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	awsuser "github.com/aws/aws-sdk-go/service/iam"
	// "github.com/aws/aws-sdk-go-v2/service/iam"

	"github.com/golang/glog"
	"github.com/yard-turkey/lib-bucket-provisioner/pkg/apis/objectbucket.io/v1alpha1"
	libbkt "github.com/yard-turkey/lib-bucket-provisioner/pkg/provisioner"
	apibkt "github.com/yard-turkey/lib-bucket-provisioner/pkg/provisioner/api"
	bkterr "github.com/yard-turkey/lib-bucket-provisioner/pkg/provisioner/api/errors"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	restclient "k8s.io/client-go/rest"
	storage "k8s.io/api/storage/v1"

	//"github.com/crossplane/pkg/controller/storage/bucket"
	//"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/arn"
)

const (
	defaultRegion	= "us-west-1"
	httpPort	= 80
	httpsPort	= 443
	provisionerName	= "aws-s3.io/bucket"
	regionInsert	= "<REGION>"
	s3Hostname	= "s3-"+regionInsert+".amazonaws.com"
	s3BucketArn      = "arn:aws:s3:::%s"
	policyArn = "arn:aws:iam::%s:policy/%s"
	bucketUserName = "screeleytest"
	createBucketUser = false
)

var (
	kubeconfig string
	masterURL  string
)

type awsS3Provisioner struct {
	region	  string
	// session is the aws session
	session	  *session.Session
	// svc is the aws s3 service based on the session
	svc	  *s3.S3
	// iam client
	iamservice *awsuser.IAM
	//kube client
	clientset *kubernetes.Clientset
	// access keys for aws acct for the bucket *owner*
	bktOwnerAccessId  string
	bktOwnerSecretKey string
	bktCreateUser     string
	bktUserName       string
	bktUserAccessId   string
	bktUserSecretKey  string
	bktUserAccountId  string
	bktUserPolicyArn  string
}

func NewAwsS3Provisioner(cfg *restclient.Config, s3Provisioner awsS3Provisioner) *libbkt.ProvisionerController {

	opts := &libbkt.ProvisionerOptions{}
	return libbkt.NewProvisioner(cfg, provisionerName, s3Provisioner, opts)
}

// PolicyDocument is the structure of IAM policy document
type PolicyDocument struct {
	Version   string
	Statement []StatementEntry
}

// StatementEntry is used to define permission statements in a PolicyDocument
type StatementEntry struct {
	Sid      string
	Effect   string
	Action   []string
	Resource []string
}

func (p awsS3Provisioner)createBucketPolicyDocument(options *apibkt.BucketOptions) (string, error) {
	bucketARN := fmt.Sprintf(s3BucketArn, options.BucketName)
	glog.Infof("createBucketPolicyDocument - bucketARN = %s", bucketARN)

	read := StatementEntry{
		Sid:       "s3Read",
		Effect:    "Allow",
		Action: []string{
			"s3:Get*",
			"s3:List*",
		},
		Resource: []string{bucketARN + "/*"},
	}

	//TODO uncomment after implement the switch below
	//TODO default should be ?
	write := StatementEntry{
		Sid:       "s3Write",
		Effect:    "Allow",
		Action: []string{
			"s3:DeleteObject",
			"s3:Put*",
		},
		Resource: []string{bucketARN + "/*"},
	}

	policy := PolicyDocument{
		Version:   "2012-10-17",
		Statement: []StatementEntry{},
	}

	// do a switch case here to figure out which policy to include
	// for now we are commenting until we can update the lib
	// this will come from bucketOptions I'm guessing (obc or sc params)?
	/*
	if spec.LocalPermission != nil {
		switch *spec.LocalPermission {
		case storage.ReadOnlyPermission:
			policy.Statement = append(policy.Statement, read)
		case storage.WriteOnlyPermission:
			policy.Statement = append(policy.Statement, write)
		case storage.ReadWritePermission:
			policy.Statement = append(policy.Statement, read, write)
		default:
			return "", fmt.Errorf("unknown permission, %s", *spec.LocalPermission)
		}
	}
    */
	//For now hard coding read and write
	policy.Statement = append(policy.Statement, read, write)
	//policy.Statement = append(policy.Statement, read)

	b, err := json.Marshal(&policy)
	if err != nil {
		return "", fmt.Errorf("error marshaling policy, %s", err.Error())
	}

	return string(b), nil
}

func (p awsS3Provisioner) createUserPolicy(iamsvc *awsuser.IAM, policyName string, policyDocument string) (*awsuser.CreatePolicyOutput, error) {

	policyInput := &awsuser.CreatePolicyInput{
		PolicyName: aws.String(policyName),
		PolicyDocument: aws.String(policyDocument),
	}

	result, err := iamsvc.CreatePolicy(policyInput)
	if err != nil {
		fmt.Println("Error", err)
		return nil, err
	}

	glog.Infof("createUserPolicy %s successfully created", policyName)
	return result, nil
}

func (p *awsS3Provisioner) getPolicyARN(policyName string) (string, error) {
	accountID, err := p.getAccountID()
	if err != nil {
		return "", err
	}
	//set the accountID in our provisioner
	p.bktUserAccountId = accountID
	policyARN := fmt.Sprintf(policyArn, accountID, policyName)
	//set the policyARN for our provisioner
	p.bktUserPolicyArn = policyARN
	glog.Infof("getPolicyARN %s for Account ID %s for Policy %s", policyARN, accountID, policyName)
	return policyARN, nil
}

func (p awsS3Provisioner) attachPolicyToUser(policyName string, username string) error {
	policyArn, err := p.getPolicyARN(policyName)
	if err != nil {
		return err
	}
	_, err = p.iamservice.AttachUserPolicy(&awsuser.AttachUserPolicyInput{PolicyArn: aws.String(policyArn), UserName: aws.String(p.bktUserName)})
	return err
}

// getAccountID - Gets the accountID of the authenticated session.
func (p *awsS3Provisioner) getAccountID() (string, error) {
	//TODO - right now our authenticated session is from our provider (i.e. StorageClass)
	//       do we need to create a new session with our *new* user
	//       and get the accountID associated with that - maybe it's all the same
	user, err := p.iamservice.GetUser(&awsuser.GetUserInput{
		UserName: &p.bktUserName,})
	if err != nil {
		glog.Errorf("Could not get new user %s", p.bktUserName)
		return "", err
	}
	arnData, err := arn.Parse(*user.User.Arn)
	if err != nil {
		return "", err
	}
	glog.Infof("New User %s and AccountID %s", p.bktUserName, aws.StringValue(&arnData.AccountID))
	return aws.StringValue(&arnData.AccountID), nil
}

// Create IAM User and pass back accessKeys
func (p *awsS3Provisioner)createIAMUser(user string) (string, string, error){
	//Create an iam user to pass back as bucket creds???

	myuser := p.bktUserName
	if len(user) != 0 {
		myuser = user
		p.bktUserName = user
	}
	glog.Infof("in createIAM - user %s", myuser)

	//Create IAM service (maybe this should be added into our default or obc session
	//or create all services type of function?
	iamsvc := awsuser.New(p.session)
	p.iamservice = iamsvc

	//Create the new user
	uresult, uerr := iamsvc.CreateUser(&awsuser.CreateUserInput{
		UserName: &myuser,
	})
	if uerr != nil {
		glog.Errorf("error creating user %v", uerr)
		return "", "", uerr
	}

	// print out successful result
	glog.Infof("Successfully created iam user %v", uresult)

	//Create the Access Keys for the new user
	aresult, aerr := iamsvc.CreateAccessKey(&awsuser.CreateAccessKeyInput{
		UserName: &myuser,
	})
	if aerr != nil {
		//print error message
		glog.Errorf("error creating accessKey %v", aerr)
	}
	glog.Infof("Successfully created Access Keys for user %s %v", myuser, aresult)
	// print out successful result for testing
	// and populate our receiver
	glog.Infof("Summary of successful created iam user %s", myuser)
	myaccesskey := aws.StringValue(aresult.AccessKey.AccessKeyId)
	p.bktUserAccessId = myaccesskey
	glog.Infof("accessKey = %s", myaccesskey)
	mysecretaccesskey := aws.StringValue(aresult.AccessKey.SecretAccessKey)
	p.bktUserSecretKey = mysecretaccesskey
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

func (p *awsS3Provisioner) createConnection(name, accessId, secretKey string) (*v1alpha1.Connection) {
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
				AccessKeyId:     accessId,
				SecretAccessKey: secretKey,
			},
		},
	}
}

func (p awsS3Provisioner) createBucket(name string) (error) {

	bucketinput := &s3.CreateBucketInput{
		Bucket: &name,
	}

	_, err := p.svc.CreateBucket(bucketinput)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case s3.ErrCodeBucketAlreadyExists:
				msg := fmt.Sprintf("Bucket %q already exists", name)
				glog.Errorf(msg)
				return bkterr.NewBucketExistsError(msg)
			case s3.ErrCodeBucketAlreadyOwnedByYou:
				msg := fmt.Sprintf("Bucket %q already owned by you", name)
				glog.Errorf(msg)
				return bkterr.NewBucketExistsError(msg)
			}
		}
		return fmt.Errorf("Bucket %q could not be created: %v", name, err)
	}
	glog.Infof("Bucket %s successfully created", name)

	//Now at this point, we have a bucket and an owner
	//we should now create the user for the bucket
	return nil
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

// Get the secret namespace and name from the passed in map.
// Empty strings are also returned.
func (p *awsS3Provisioner)setCreateBucketUserOptions(obc *v1alpha1.ObjectBucketClaim) error {

	const (
		scBucketUser = "createBucketUser"
	)
	p.bktUserAccessId = ""
	p.bktUserSecretKey = ""


	sc, err := p.getClassForBucketClaim(obc)
	if err != nil {
		glog.Errorf("Get failed for storage class %q", obc.Spec.StorageClassName)
		p.bktCreateUser = "yes"
		return err
	}

	// get expected sc parameters containing region and secret
	doCreateUser, ok := sc.Parameters[scBucketUser]
	if !ok || doCreateUser == "no" {
		glog.Infof("setCreateBucketUserOptions - did not find StorageClass flag to create user %s", scBucketUser)
		p.bktCreateUser = "no"
		return nil
	}
	p.bktCreateUser = "yes"

	return nil
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

// Handle Policy and User Creation if
// flag is set
func (p awsS3Provisioner) handleUserAndPolicy(options *apibkt.BucketOptions) (string, string, error) {
	userAccessId := p.bktUserAccessId
	userSecretKey := p.bktUserSecretKey

	userAccessId, userSecretKey, err := p.createIAMUser("")
	if err != nil {
		//should we fail here or keep going?
		glog.Errorf("error creating IAMUser %s: %v", options.BucketName, err)
		return "", "", err
	}

	//Create the Policy for the user + bucket
	//if createBucket was successful
	//might change the input param into this function, we need bucketName
	//and maybe accessPerms (read, write, read/write)
	policyDoc, err := p.createBucketPolicyDocument(options)
	if err != nil {
		//We did get our user created, but not our policy doc
		//I'm going to pass back our user for now
		glog.Errorf("error creating policyDoc %s: %v", options.BucketName, err)
		return userAccessId, userSecretKey, err
	}

	//Create the policy in aws for the user and bucket
	_, perr := p.createUserPolicy(p.iamservice, options.BucketName, policyDoc)
	if perr != nil {
		//should we fail here or keep going?
		glog.Errorf("error creating userPolicy for %s on bucket %s: %v", p.bktUserName, options.BucketName, perr)
		return userAccessId, userSecretKey, err
	}

	//attach policy to user
	aperr := p.attachPolicyToUser(options.BucketName, p.bktUserName)
	if aperr != nil {
		glog.Errorf("error attaching userPolicy for %s on bucket %s: %v", p.bktUserName, options.BucketName, aperr)
		return userAccessId, userSecretKey, err
	}

	return userAccessId, userSecretKey, nil
}

func (p *awsS3Provisioner) setBucketUser(name string){
    if len(name) == 0 {
    	p.bktUserName = bucketUserName
	}
	p.bktUserName = name
}

// Provision creates an aws s3 bucket and returns a connection info
// representing the bucket's endpoint and user access credentials.
func (p awsS3Provisioner) Provision(options *apibkt.BucketOptions) (*v1alpha1.Connection, error) {

	obc := options.ObjectBucketClaim
	serr := p.setCreateBucketUserOptions(obc)
	if serr != nil {
		//letting it keep going if there is some strange error here, shouldn't be
		//just won't do any user creation
		glog.Errorf("error setting Bucket User Options %q: %v", options.BucketName, serr)
		p.bktCreateUser = "no"
	}

	// set the aws session from the obc in the receiver
	err := p.awsSessionFromOBC(obc)
	if err != nil {
		return nil, fmt.Errorf("error creating session from OBC \"%s/%s\": %v", obc.Namespace, obc.Name, err)
	}

	glog.Infof("Creating S3 service for OBC \"%s/%s\"", obc.Namespace, obc.Name)
	p.svc = s3.New(p.session)

	//Create the bucket
	glog.Infof("Creating bucket %q", options.BucketName)
	berr := p.createBucket(options.BucketName)
	if berr != nil {
		glog.Errorf("error creating bucket %q: %v", options.BucketName, berr)
		return nil, fmt.Errorf("error creating bucket %q: %v", options.BucketName, berr)
	}

	//createBucket was successful at this point
	//we can create a new IAM user for access
	//and attach policy to bucket and user
	//Create New IAM User for the bucket
	if p.bktCreateUser == "yes" {
		//set user
		p.setBucketUser(options.BucketName)

		//handle all iam and policy operations
		userAccessId, userSecretKey, err := p.handleUserAndPolicy(options)
		if err != nil {
			//what to do - something failed along the way
			//do we fall back and create our connection with
			//the default bktOwnerRef?
			glog.Errorf("Something failed along the way for handling Users and Policy %v", err)
			return p.createConnection(options.BucketName, p.bktOwnerAccessId, p.bktOwnerSecretKey), err
		}
		return p.createConnection(options.BucketName, userAccessId, userSecretKey), nil
	}

	//Now create the connection
	return p.createConnection(options.BucketName, p.bktOwnerAccessId, p.bktOwnerSecretKey), nil
}


// Delete the bucket and all its objects.
// Note: only called when the bucket's reclaim policy is "delete".
func (p awsS3Provisioner) Delete(ob *v1alpha1.ObjectBucket) error {
	//TODO clean up dynamic user + policy

	bktName := ob.Spec.Endpoint.BucketName
	iter := s3manager.NewDeleteListIterator(p.svc, &s3.ListObjectsInput{
		Bucket: aws.String(bktName),
	})

	glog.Infof("Deleting all objects in bucket %q (from OB %q)", bktName, ob.Name)
	err := s3manager.NewBatchDeleteWithClient(p.svc).Delete(aws.BackgroundContext(), iter)
	if err != nil {
		return fmt.Errorf("Error deleting objects from bucket %q: %v", bktName,  err)
	}

	glog.Infof("Deleting empty bucket %q from OB %q", bktName, ob.Name)
	_, err = p.svc.DeleteBucket(&s3.DeleteBucketInput{
    		Bucket: aws.String(bktName),
	})
	if err != nil {
		return fmt.Errorf("Error deleting empty bucket %q: %v", bktName,  err)
	}

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
