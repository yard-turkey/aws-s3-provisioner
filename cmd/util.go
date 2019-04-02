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
	"math/rand"
	_ "net/url"

	"github.com/yard-turkey/lib-bucket-provisioner/pkg/apis/objectbucket.io/v1alpha1"
	storageV1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Return the storage class for a given name.
func (p *awsS3Provisioner) getClassByNameForBucket(className string) (*storageV1.StorageClass, error) {

	if className == "" {
		//TODO: retry?
		return nil, fmt.Errorf("cannot Get StorageClass: name is blank")
	}

	class, err := p.clientset.StorageV1().StorageClasses().Get(className, metav1.GetOptions{})
	// TODO: retry w/ exponential backoff
	if err != nil {
		return nil, fmt.Errorf("unable to Get storageclass %q: %v", className, err)
	}
	return class, nil
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
		// TODO: some kind of exponential backoff and retry...
		return fmt.Errorf("unable to get Secret \"%s/%s\": %v", ns, name, err)
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

func randomString(n int) string {
	var letterRunes = []rune("0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

func createUserName(bkt string) string {
	// prefix is bucket name
	if len(bkt) > maxBucketLen  {
		bkt = bkt[:(maxBucketLen-1)]
	}
	return fmt.Sprintf("%s-%s", bkt, randomString(genUserLen))
}
