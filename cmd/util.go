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
	"strings"
	"time"

	"github.com/golang/glog"
	"github.com/kube-object-storage/lib-bucket-provisioner/pkg/apis/objectbucket.io/v1alpha1"
	storageV1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	v1 "k8s.io/api/core/v1"
)

const (
	// Security limits for YAML processing
	maxYAMLSize = 1024 * 1024 // 1MB limit for YAML content
	maxAliasDepth = 50        // Limit alias depth to prevent unbounded expansion
)

// validateYAMLInput validates YAML content before processing to prevent DoS attacks
func validateYAMLInput(data []byte) error {
	if len(data) > maxYAMLSize {
		return fmt.Errorf("YAML content exceeds maximum size limit of %d bytes", maxYAMLSize)
	}
	
	// Check for excessive alias usage patterns that could indicate malicious input
	aliasCount := strings.Count(string(data), "&")
	aliasRefCount := strings.Count(string(data), "*")
	
	if aliasCount > maxAliasDepth || aliasRefCount > maxAliasDepth*2 {
		return fmt.Errorf("YAML content contains suspicious alias patterns (aliases: %d, refs: %d)", aliasCount, aliasRefCount)
	}
	
	return nil
}

// Return the storage class for a given name.
func (p *awsS3Provisioner) getClassByNameForBucket(className string) (*storageV1.StorageClass, error) {

	glog.V(2).Infof("getting storage class %q...", className)
	class, err := p.clientset.StorageV1().StorageClasses().Get(className, metav1.GetOptions{})
	// TODO: retry w/ exponential backoff
	if err != nil {
		return nil, fmt.Errorf("unable to Get storageclass %q: %v", className, err)
	}
	return class, nil
}

// Return the region name from the passed in storage class.
func getRegion(sc *storageV1.StorageClass) string {

	const scRegionKey = "region"
	return sc.Parameters[scRegionKey]
}

// Return the secret namespace and name from the passed storage class.
func getSecretName(sc *storageV1.StorageClass) (string, string) {

	const (
		scSecretNameKey = "secretName"
		scSecretNSKey   = "secretNamespace"
	)
	return sc.Parameters[scSecretNSKey], sc.Parameters[scSecretNameKey]
}

// Get the secret and set the receiver to the accessKeyId and secretKey.
func (p *awsS3Provisioner) credsFromSecret(c *kubernetes.Clientset, ns, name string) error {

	nsName := fmt.Sprintf("%s/%s", ns, name)
	glog.V(2).Infof("getting secret %q...", nsName)
	secret, err := c.CoreV1().Secrets(ns).Get(name, metav1.GetOptions{})
	if err != nil {
		// TODO: some kind of exponential backoff and retry...
		return fmt.Errorf("unable to get Secret %q: %v", nsName, err)
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

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	var letterRunes = []rune("0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[r.Intn(len(letterRunes))]
	}
	return string(b)
}

func (p *awsS3Provisioner) createUserName(bkt string) string {
	// prefix is bucket name
	if len(bkt) > maxBucketLen  {
		bkt = bkt[:(maxBucketLen-1)]
	}

	userbool := true
	name := ""
	i := 0
	for ok := true; ok; ok = userbool {
		name = fmt.Sprintf("%s-%s", bkt, randomString(genUserLen))
		userbool = p.checkIfUserExists(name)
		i++
	}
	glog.V(2).Infof("Generated user %s after %v iterations", name, i)
	return name
}
