package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

type mapper interface {
	toMap() map[string]string
}

// Exported constants used by provisioners: including conventional
// environment variable names for S3 Access and Secret Key, and
// map key names. Eg. key to access a bucket name in a storage class
// used for brownfield buckets, or the key to create an OB's
// Authentication{}.
const (
	AwsKeyField        = "AWS_ACCESS_KEY_ID"
	AwsSecretField     = "AWS_SECRET_ACCESS_KEY"
	StorageClassBucket = "BUCKET_NAME"
)

// AccessKeys is an Authentication type for passing AWS S3 style key pairs from the provisioner to the reconciler
type AccessKeys struct {
	// AccessKeyId is the S3 style access key to be written to a secret
	AccessKeyID string `json:"-"`
	// SecretAccessKey is the S3 style secret key to be written to a secret
	SecretAccessKey string `json:"-"`
}

var _ mapper = &AccessKeys{}

func (ak *AccessKeys) toMap() map[string]string {
	return map[string]string{
		AwsKeyField:    ak.AccessKeyID,
		AwsSecretField: ak.SecretAccessKey,
	}
}

// Authentication wraps all supported auth types.  The design choice enables expansion of supported types while
// protecting backwards compatibility.
type Authentication struct {
	AccessKeys           *AccessKeys       `json:"-"`
	AdditionalSecretData map[string]string `json:"-"`
}

// ToMap converts the any defined authentication type into a map[string]string for writing to a Secret.StringData field
func (a *Authentication) ToMap() map[string]string {
	if a == nil {
		return map[string]string{}
	}
	if a.AccessKeys != nil {
		return a.AccessKeys.toMap()
	}
	return map[string]string{}
}

// Endpoint contains all connection relevant data that an app may require for accessing
// the bucket
type Endpoint struct {
	BucketHost           string            `json:"bucketHost"`
	BucketPort           int               `json:"bucketPort"`
	BucketName           string            `json:"bucketName"`
	Region               string            `json:"region"`
	SubRegion            string            `json:"subRegion"`
	SSL                  bool              `json:"ssl"`
	AdditionalConfigData map[string]string `json:"additionalConfig"`
}

// Connection encapsulates Endpoint and Authentication data to simplify the expected return values of the Provision()
// interface method.  This makes it more clear to library consumers what specific values they should return from their
// Provisioner interface implementation.
type Connection struct {
	Endpoint        *Endpoint         `json:"endpoint"`
	Authentication  *Authentication   `json:"-"`
	AdditionalState map[string]string `json:"additionalState"`
}

// ObjectBucketSpec defines the desired state of ObjectBucket. Fields defined here should be normal among all providers.
// Authentication must be of a type defined in this package to pass type checks in reconciler
type ObjectBucketSpec struct {
	StorageClassName string                                `json:"storageClassName"`
	ReclaimPolicy    *corev1.PersistentVolumeReclaimPolicy `json:"reclaimPolicy"`
	ClaimRef         types.UID                             `json:"claimRef"`
	*Connection      `json:"Connection"`
}

// ObjectBucketStatusPhase is set by the controller to save the state of the provisioning process.
type ObjectBucketStatusPhase string

const (
	// ObjectBucketStatusPhaseBound indicates that the objectBucket has been logically bound to a claim following a
	// successful provision.  It is NOT the authority for the status of the claim an object bucket. For that, see
	// objectBucketClaim.Spec.ObjectBucketName
	ObjectBucketStatusPhaseBound ObjectBucketStatusPhase = "bound"
	// ObjectBucketStatusPhaseReleased indicates that the object bucket was once bound to a claim that has since been deleted
	// this phase can occur when the claim is deleted and the reconciler is in the process of either deleting the bucket or
	// revoking access to that bucket in the case of brownfield.
	ObjectBucketStatusPhaseReleased ObjectBucketStatusPhase = "released"
	// ObjectBucketStatusPhaseFailed TODO this phase does not have a defined reason for existing.  If provisioning fails
	//  the OB is cleaned up.  Since we generate OBs for brownfield cases, we also would delete them on failures.  The
	//  result is that if this phase is set, the OB would deleted soon after anyway.
	ObjectBucketStatusPhaseFailed ObjectBucketStatusPhase = "failed"
)

// ObjectBucketStatus defines the observed state of ObjectBucket
type ObjectBucketStatus struct {
	Phase      ObjectBucketStatusPhase `json:"phase"`
	Conditions corev1.ConditionStatus  `json:"conditions"`
}

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:openapi-gen=true

// ObjectBucket is the Schema for the objectbuckets API
type ObjectBucket struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ObjectBucketSpec   `json:"spec,omitempty"`
	Status ObjectBucketStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ObjectBucketList contains a list of ObjectBucket
type ObjectBucketList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ObjectBucket `json:"items"`
}
