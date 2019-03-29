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
	storageV1 "k8s.io/api/storage/v1"
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

// Handle Policy and User Creation when flag is set.
// Note: new user name is the same as the bucket name.
func (p *awsS3Provisioner) handleUserAndPolicy(options *apibkt.BucketOptions) (string, string, error) {

	userAccessId, userSecretKey, err := p.createIAMUser("")
	if err != nil {
		//should we fail here or keep going?
		glog.Errorf("error creating IAM user %s: %v", options.BucketName, err)
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
	_, err = p.createUserPolicy(p.iamsvc, options.BucketName, policyDoc)
	if err != nil {
		//should we fail here or keep going?
		glog.Errorf("error creating userPolicy for user %q on bucket %q: %v", p.bktUserName, options.BucketName, err)
		return userAccessId, userSecretKey, err
	}

	//attach policy to user
	err = p.attachPolicyToUser(options.BucketName, p.bktUserName)
	if err != nil {
		glog.Errorf("error attaching userPolicy for user %q on bucket %q: %v", p.bktUserName, options.BucketName, err)
		return userAccessId, userSecretKey, err
	}

	return userAccessId, userSecretKey, nil
}

func (p *awsS3Provisioner) handleUserAndPolicyDeletion(name string) error {

	//TODO we need to store and retrieve policyArn - now using hardcoded tmpArn
	glog.Infof("In handleUserandPolicyDeletion")
	iamsvc := awsuser.New(p.session)
	p.iamsvc = iamsvc
	tmpArn := "arn:aws:iam::939345161466:policy/screeley-provb-5"

	glog.Infof("about to delete policy %s", tmpArn)

	// Detach Policy
	_, err := p.iamsvc.DetachUserPolicy((&awsuser.DetachUserPolicyInput{PolicyArn: aws.String(tmpArn),UserName: aws.String(name)}))
	if err != nil {
		// Not sure we want to stop the deletion of the user or bucket at this point
		// so just logging an error
		glog.Errorf("Error detaching User Policy %s %v", tmpArn, err)
		return err
	}
	glog.Infof("successfully detached policy %s", tmpArn)

	// Delete Policy
	_, err = p.iamsvc.DeletePolicy(&awsuser.DeletePolicyInput{PolicyArn: aws.String(tmpArn)})
	if err != nil {
		// Not sure we want to stop the deletion of the user or bucket at this point
		// so just logging an error
		glog.Errorf("Error deleting User Policy %s %v", tmpArn, err)
		return err
	}
	glog.Infof("successfully deleted policy %s", tmpArn)

	// Delete AccessKeys
	accessKeyId, err := p.getAccessKey(name)
	if len(accessKeyId) != 0 {
		glog.Infof("access key id is %s", accessKeyId)
		_, err = p.iamsvc.DeleteAccessKey(&awsuser.DeleteAccessKeyInput{AccessKeyId: aws.String(accessKeyId),UserName: aws.String(name)})
		if err != nil {
			// Not sure we want to stop the deletion of the user or bucket at this point
			// so just logging an error
			glog.Errorf("Error deleting access key for user %s %v", name, err)
			return err
		}
		glog.Infof("successfully deleted policy %s", tmpArn)
	}

	// Delete IAM User
	glog.Infof("Deleting User %s", name)
	_, err = p.iamsvc.DeleteUser(&awsuser.DeleteUserInput{UserName: aws.String(name)})
	if err != nil {
		// Not sure we want to stop the deletion of the user or bucket at this point
		// so just logging an error
		glog.Errorf("Error deleting User %s %v", name, err)
		return err
	}

	glog.Infof("successfully deleted policy and user")
	return err
}

func (p *awsS3Provisioner) createBucketPolicyDocument(options *apibkt.BucketOptions) (string, error) {

	bucketARN := fmt.Sprintf(s3BucketArn, options.BucketName)
	glog.V(2).Infof("createBucketPolicyDocument - bucketARN = %s", bucketARN)

	read := StatementEntry{
		Sid:    "s3Read",
		Effect: "Allow",
		Action: []string{
			"s3:Get*",
			"s3:List*",
		},
		Resource: []string{bucketARN + "/*"},
	}
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
			case storageV1.ReadOnlyPermission:
				policy.Statement = append(policy.Statement, read)
			case storageV1.WriteOnlyPermission:
				policy.Statement = append(policy.Statement, write)
			case storageV1.ReadWritePermission:
				policy.Statement = append(policy.Statement, read, write)
			default:
				return "", fmt.Errorf("unknown permission, %s", *spec.LocalPermission)
			}
		}
	*/
	//For now hard coding read and write
	policy.Statement = append(policy.Statement, read, write)

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

	glog.V(2).Infof("createUserPolicy %s successfully created", policyName)
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
	glog.V(2).Infof("getPolicyARN %s for Account ID %s for Policy %s", policyARN, accountID, policyName)
	return policyARN, nil
}

func (p *awsS3Provisioner) attachPolicyToUser(policyName string, username string) error {

	policyARN, err := p.getPolicyARN(policyName)
	if err != nil {
		return err
	}

	_, err = p.iamsvc.AttachUserPolicy(&awsuser.AttachUserPolicyInput{PolicyArn: aws.String(policyARN), UserName: aws.String(p.bktUserName)})
	if err == nil {
		glog.Infof("Successfully attached Policy %s to User %s", policyName, p.bktUserName)
	}
	return err
}

// getAccountID - Gets the accountID of the authenticated session.
func (p *awsS3Provisioner) getAccountID() (string, error) {

	user, err := p.iamsvc.GetUser(&awsuser.GetUserInput{
		UserName: &p.bktUserName})
	if err != nil {
		glog.Errorf("Could not get new user %s", p.bktUserName)
		return "", err
	}
	arnData, err := arn.Parse(*user.User.Arn)
	if err != nil {
		return "", err
	}
	glog.V(2).Infof("New User %s and AccountID %s", p.bktUserName, aws.StringValue(&arnData.AccountID))
	return aws.StringValue(&arnData.AccountID), nil
}

// getAccessKeyId - Gets the accountID of the authenticated session.
func (p *awsS3Provisioner) getAccessKey(username string) (string, error) {

	keys, err := p.iamsvc.ListAccessKeys(&awsuser.ListAccessKeysInput{UserName: aws.String(username)})
	if err != nil {
		glog.Errorf("Could not get access key for new user %s", username)
		return "", err
	}
	glog.Infof("AccessKeyMeta %+v", keys)

	for _, keys := range keys.AccessKeyMetadata {
		return aws.StringValue(keys.AccessKeyId), nil
	}

	return "", nil
}

// Create dyanamic IAM user, pass back accessKeys and set user name in receiver.
// The user name is set to 1) passed-in user string, 2) receiver's user name
// field, 3) bucket name.
func (p *awsS3Provisioner) createIAMUser(user string) (string, string, error) {

	myuser := user
	if len(myuser) == 0 {
		myuser = p.bktUserName
	}
	if len(myuser) == 0 {
		myuser = p.bucketName
	}
	p.bktUserName = myuser
	glog.V(2).Infof("in createIAM - user %s", myuser)

	//Create IAM service (maybe this should be added into our default or obc session
	//or create all services type of function?
	iamsvc := awsuser.New(p.session)
	p.iamsvc = iamsvc

	//Create the new user
	uresult, err := iamsvc.CreateUser(&awsuser.CreateUserInput{
		UserName: &myuser,
	})
	if err != nil {
		glog.Errorf("error creating user %v", err)
		return "", "", err
	}

	// print out successful result
	glog.V(2).Infof("Successfully created iam user %v", uresult)

	//Create the Access Keys for the new user
	aresult, err := iamsvc.CreateAccessKey(&awsuser.CreateAccessKeyInput{
		UserName: &myuser,
	})
	if err != nil {
		glog.Errorf("error creating accessKey %v", err)
		return "", "", err
	}

	glog.V(2).Infof("Successfully created Access Keys for user %s: %v", myuser, aresult)
	// print out successful result for testing
	// and populate our receiver
	p.bktUserAccessId = aws.StringValue(aresult.AccessKey.AccessKeyId)
	p.bktUserSecretKey = aws.StringValue(aresult.AccessKey.SecretAccessKey)
	glog.V(2).Infof("Summary of successfully created IAM user %q:\n   accessKey=%s\n   secretAccessKey=%s", myuser, p.bktUserAccessId, p.bktUserSecretKey)

	return p.bktUserAccessId, p.bktUserSecretKey, nil
}

// Get StorageClass from OBC and check params for createBucketUser and set
// provisioner receiver field.
func (p *awsS3Provisioner) setCreateBucketUserOptions(obc *v1alpha1.ObjectBucketClaim, sc *storageV1.StorageClass) {

	const scBucketUser = "createBucketUser"

	// get sc user-access flag parameter
	doCreateUser, ok := sc.Parameters[scBucketUser]
	if !ok {
		glog.V(2).Infof("setCreateBucketUserOptions - did not find StorageClass flag to create user %s - defaulting to create user", scBucketUser)
		p.bktCreateUser = "yes"
		return
	}
	if doCreateUser == "no" {
		glog.V(2).Infof("setCreateBucketUserOptions - did find StorageClass flag to not create user %s", scBucketUser)
		p.bktCreateUser = "no"
		return
	}

	p.bktCreateUser = "yes"
	return
}
