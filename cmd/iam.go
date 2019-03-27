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
	"fmt"
	_ "net/url"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
	awsuser "github.com/aws/aws-sdk-go/service/iam"
	"github.com/golang/glog"
	"github.com/yard-turkey/lib-bucket-provisioner/pkg/apis/objectbucket.io/v1alpha1"
	apibkt "github.com/yard-turkey/lib-bucket-provisioner/pkg/provisioner/api"
)

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

func (p awsS3Provisioner) createBucketPolicyDocument(options *apibkt.BucketOptions) (string, error) {
	bucketARN := fmt.Sprintf(s3BucketArn, options.BucketName)
	glog.Infof("createBucketPolicyDocument - bucketARN = %s", bucketARN)

	read := StatementEntry{
		Sid:    "s3Read",
		Effect: "Allow",
		Action: []string{
			"s3:Get*",
			"s3:List*",
		},
		Resource: []string{bucketARN + "/*"},
	}

	//TODO uncomment after implement the switch below
	//TODO default should be ?
	write := StatementEntry{
		Sid:    "s3Write",
		Effect: "Allow",
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
		PolicyName:     aws.String(policyName),
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
		UserName: &p.bktUserName})
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

// Create Dyanamic IAM User and pass back accessKeys
func (p *awsS3Provisioner) createIAMUser(user string) (string, string, error) {
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

// Get StorageClass from OBC and check params
// for createBucketUser and set provisioner.
func (p *awsS3Provisioner) setCreateBucketUserOptions(obc *v1alpha1.ObjectBucketClaim) error {

	const (
		scBucketUser = "createBucketUser"
	)
	//p.bktUserAccessId = ""
	//p.bktUserSecretKey = ""

	sc, err := p.getClassForBucketClaim(obc)
	if err != nil {
		glog.Errorf("Get failed for storage class %q", obc.Spec.StorageClassName)
		p.bktCreateUser = "yes"
		return err
	}

	// get expected sc parameters containing region and secret
	doCreateUser, ok := sc.Parameters[scBucketUser]
	if !ok {
		glog.Infof("setCreateBucketUserOptions - did not find StorageClass flag to create user %s - defaulting to create user", scBucketUser)
		p.bktCreateUser = "yes"
		return nil
	}
	if doCreateUser == "no" {
		glog.Infof("setCreateBucketUserOptions - did find StorageClass flag to not create user %s", scBucketUser)
		p.bktCreateUser = "no"
		return nil
	}
	p.bktCreateUser = "yes"

	return nil
}
