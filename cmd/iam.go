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
	"github.com/kube-object-storage/lib-bucket-provisioner/pkg/apis/objectbucket.io/v1alpha1"
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

// handleUserAndPolicy takes care of policy and user creation when flag is set.
func (p *awsS3Provisioner) handleUserAndPolicy(bktName string) (string, string, error) {

	glog.V(2).Infof("creating user and policy for bucket %q", bktName)

	userAccessId, userSecretKey, err := p.createIAMUser("")
	uname := p.bktUserName
	if err != nil {
		//should we fail here or keep going?
		glog.Errorf("error creating IAM user %q: %v", uname, err)
		return "", "", err
	}

	//Create the Policy for the user + bucket
	//if createBucket was successful
	//might change the input param into this function, we need bucketName
	//and maybe accessPerms (read, write, read/write)
	policyDoc, err := p.createBucketPolicyDocument(bktName)
	if err != nil {
		//We did get our user created, but not our policy doc
		//I'm going to pass back our user for now
		glog.Errorf("error creating policyDoc %s: %v", bktName, err)
		return userAccessId, userSecretKey, err
	}

	// Create the policy in aws for the user and bucket
	// policyName is same as username
	_, err = p.createUserPolicy(p.iamsvc, uname, policyDoc)
	if err != nil {
		//should we fail here or keep going?
		glog.Errorf("error creating userPolicy for user %q on bucket %q: %v", uname, bktName, err)
		return userAccessId, userSecretKey, err
	}

	//attach policy to user - policyName and username are same
	err = p.attachPolicyToUser(uname)
	if err != nil {
		glog.Errorf("error attaching userPolicy for user %q on bucket %q: %v", uname, bktName, err)
		return userAccessId, userSecretKey, err
	}

	glog.V(2).Infof("successfully created user and policy for bucket %q", bktName)
	return userAccessId, userSecretKey, nil
}

func (p *awsS3Provisioner) handleUserAndPolicyDeletion(bktName string) error {

	glog.V(2).Infof("deleting user and policy for bucket %q", bktName)

	uname := p.bktUserName
	p.iamsvc = awsuser.New(p.session)
	arn := p.bktUserPolicyArn

	// Detach Policy
	_, err := p.iamsvc.DetachUserPolicy((&awsuser.DetachUserPolicyInput{PolicyArn: aws.String(arn), UserName: aws.String(uname)}))
	if err != nil {
		// Not sure we want to stop the deletion of the user or bucket at this point
		// so just logging an error
		glog.Errorf("Error detaching User Policy %s %v", arn, err)
		return err
	}
	glog.V(2).Infof("successfully detached policy %q, user %q", arn, uname)

	// Delete Policy
	_, err = p.iamsvc.DeletePolicy(&awsuser.DeletePolicyInput{PolicyArn: aws.String(arn)})
	if err != nil {
		// Not sure we want to stop the deletion of the user or bucket at this point
		// so just logging an error
		glog.Errorf("Error deleting User Policy %s %v", arn, err)
		return err
	}
	glog.V(2).Infof("successfully deleted policy %q", arn)

	// Delete AccessKeys
	accessKeyId, err := p.getAccessKey(uname)
	if len(accessKeyId) != 0 {
		_, err = p.iamsvc.DeleteAccessKey(&awsuser.DeleteAccessKeyInput{AccessKeyId: aws.String(accessKeyId), UserName: aws.String(uname)})
		if err != nil {
			// Not sure we want to stop the deletion of the user or bucket at this point
			// so just logging an error
			glog.Errorf("Error deleting access key for user %s %v", uname, err)
			return err
		}
		glog.V(2).Infof("successfully deleted access key for user %q", uname)
	}

	// Delete IAM User
	glog.V(2).Infof("Deleting User %q", uname)
	_, err = p.iamsvc.DeleteUser(&awsuser.DeleteUserInput{UserName: aws.String(uname)})
	if err != nil {
		// Not sure we want to stop the deletion of the user or bucket at this point
		// so just logging an error
		glog.Errorf("Error deleting User %s %v", uname, err)
		return err
	}

	glog.V(2).Infof("successfully deleted user and policy for bucket %q", bktName)
	return err
}

func (p *awsS3Provisioner) createBucketPolicyDocument(bktName string) (string, error) {

	arn := fmt.Sprintf(s3BucketArn, bktName)
	p.bktUserPolicyArn = arn
	glog.V(2).Infof("createBucketPolicyDocument for bucket %q and ARN %q", bktName, arn)

	read := StatementEntry{
		Sid:    "s3Read",
		Effect: "Allow",
		Action: []string{
			"s3:Get*",
			"s3:List*",
		},
		Resource: []string{arn + "/*", arn},
	}
	write := StatementEntry{
		Sid:    "s3Write",
		Effect: "Allow",
		Action: []string{
			"s3:DeleteObject",
			"s3:Put*",
		},
		Resource: []string{arn + "/*", arn},
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

	glog.V(2).Infof("createUserPolicy %q successfully created", policyName)
	return result, nil
}

func (p *awsS3Provisioner) getPolicyARN(policyName string) (string, error) {

	glog.V(2).Infof("getting ARN for policy %q", policyName)
	accountID, err := p.getAccountID()
	if err != nil {
		return "", err
	}

	// set the accountID in our provisioner
	p.bktUserAccountId = accountID
	policyARN := fmt.Sprintf(policyArn, accountID, policyName)
	// set the policyARN for our provisioner
	p.bktUserPolicyArn = policyARN
	glog.V(2).Infof("successfully got PolicyARN %q for AccountID %s's Policy %q", policyARN, accountID, policyName)

	return policyARN, nil
}

func (p *awsS3Provisioner) attachPolicyToUser(policyName string) error {

	glog.V(2).Infof("attach policy %q to user", policyName)
	policyARN, err := p.getPolicyARN(policyName)
	if err != nil {
		return err
	}

	_, err = p.iamsvc.AttachUserPolicy(&awsuser.AttachUserPolicyInput{PolicyArn: aws.String(policyARN), UserName: aws.String(p.bktUserName)})
	if err != nil {
		return err
	}

	glog.V(2).Infof("successfully attached policy %q to user %q", policyName, p.bktUserName)
	return err
}

// getAccountID - Gets the accountID of the authenticated session.
func (p *awsS3Provisioner) getAccountID() (string, error) {

	glog.V(2).Infof("creating new user %q", p.bktUserName)
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

	glog.V(2).Infof("created user %q and accountID %q", p.bktUserName, aws.StringValue(&arnData.AccountID))
	return aws.StringValue(&arnData.AccountID), nil
}

// getAccessKeyId - Gets the accountID of the authenticated session.
func (p *awsS3Provisioner) getAccessKey(username string) (string, error) {

	glog.V(2).Infof("getting access key for user %q", username)
	keys, err := p.iamsvc.ListAccessKeys(&awsuser.ListAccessKeysInput{UserName: aws.String(username)})
	if err != nil {
		glog.Errorf("Could not get access key for new user %s", username)
		return "", err
	}

	for _, keys := range keys.AccessKeyMetadata {
		return aws.StringValue(keys.AccessKeyId), nil
	}

	glog.V(2).Infof("no access key found for user %q", username)
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
	glog.V(2).Infof("creating IAM user %q", myuser)

	//Create the new user
	_, err := p.iamsvc.CreateUser(&awsuser.CreateUserInput{
		UserName: &myuser,
	})
	if err != nil {
		glog.Errorf("error creating user %v", err)
		return "", "", err
	}

	// create the Access Keys for the new user
	aresult, err := p.iamsvc.CreateAccessKey(&awsuser.CreateAccessKeyInput{
		UserName: &myuser,
	})
	if err != nil {
		glog.Errorf("error creating accessKey %v", err)
		return "", "", err
	}

	// populate our receiver
	p.bktUserAccessId = aws.StringValue(aresult.AccessKey.AccessKeyId)
	p.bktUserSecretKey = aws.StringValue(aresult.AccessKey.SecretAccessKey)

	glog.V(2).Infof("successfully created IAM user %q with access keys", myuser)
	return p.bktUserAccessId, p.bktUserSecretKey, nil
}

// Get StorageClass from OBC and check params for createBucketUser and set
// provisioner receiver field.
func (p *awsS3Provisioner) setCreateBucketUserOptions(obc *v1alpha1.ObjectBucketClaim, sc *storageV1.StorageClass) {

	const scBucketUser = "createBucketUser"

	// get sc user-access flag parameter
	newUser, ok := sc.Parameters[scBucketUser]
	if ok && newUser == "no" {
		glog.V(2).Infof("storage class flag %q indicates to NOT create a new user", scBucketUser)
		p.bktCreateUser = "no"
		return
	}

	glog.V(2).Infof("storage class flag %s's value, or absence of flag, indicates to create a new user", scBucketUser)
	p.bktCreateUser = "yes"
	return
}
