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
	"fmt"
	_ "net/url"

	"github.com/yard-turkey/lib-bucket-provisioner/pkg/apis/objectbucket.io/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func (p *awsS3Provisioner) setBucketUser(name string) {
	if len(name) == 0 {
		p.bktUserName = bucketUserName
	}
	p.bktUserName = name
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
