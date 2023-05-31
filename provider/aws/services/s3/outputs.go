package s3

import (
	"fmt"
	"time"
)

type bucket struct {
	name         *string
	creationDate *time.Time
}
type object struct {
	key          *string
	sizeInBytes  *int64
	storageClass *string
	lastModified *time.Time
}
type bucketListOutput struct {
	buckets []*bucket
}

type bucketObjectListOutput struct {
	bucketName *string
	objects    []*object
}

type bucketInfo struct {
	bucketName               *string
	policy                   *interface{}
	policyAPIErr             error
	version                  *interface{}
	versionAPIErr            error
	tags                     *interface{}
	tagsAPIError             error
	encryptionConfig         *interface{}
	encryptionConfigAPIError error
	lifecycle                *interface{}
	lifeCycleAPIError        error
}

func (o *bucketInfo) SetBucketName(bucketName string) *bucketInfo {
	o.bucketName = &bucketName
	return o
}

func (o *bucketInfo) SetPolicy(data interface{}) *bucketInfo {
	o.policy = &data
	return o
}

func (o *bucketInfo) SetVersion(data interface{}) *bucketInfo {
	o.version = &data
	return o
}

func (o *bucketInfo) SetTags(data interface{}) *bucketInfo {
	o.tags = &data
	return o
}

func (o *bucketInfo) SetEncryptionConfig(data interface{}) *bucketInfo {
	o.encryptionConfig = &data
	return o
}
func (o *bucketInfo) SetLifeCycle(data interface{}) *bucketInfo {
	o.lifecycle = &data
	return o
}

func (o *bucketInfo) SetPolicyAPIError(err error) *bucketInfo {
	o.policyAPIErr = err
	return o
}

func (o *bucketInfo) SetVersionAPIError(err error) *bucketInfo {
	o.versionAPIErr = err
	return o
}

func (o *bucketInfo) SetTagsAPIError(err error) *bucketInfo {
	o.tagsAPIError = err
	return o
}

func (o *bucketInfo) SetEncryptionConfigAPIError(err error) *bucketInfo {
	o.encryptionConfigAPIError = err
	return o
}
func (o *bucketInfo) SetLifeCycleError(err error) *bucketInfo {
	o.lifeCycleAPIError = err
	return o
}

func (o bucketInfo) Pretty() {
	if o.encryptionConfigAPIError != nil {
		fmt.Println("encryptionConfigAPIError", o.encryptionConfigAPIError)
	} else {
		fmt.Println("encryptionConfig", *o.encryptionConfig)
	}
	if o.tagsAPIError != nil {
		fmt.Println("tagsAPIError", o.tagsAPIError)
	} else {
		fmt.Println("tags", *o.tags)
	}
	if o.policyAPIErr != nil {
		fmt.Println("policyAPIErr", o.policyAPIErr)
	} else {
		fmt.Println("policy", *o.policy)
	}
	if o.versionAPIErr != nil {
		fmt.Println("versionAPIErr", o.versionAPIErr)
	} else {
		fmt.Println("versioning", *o.version)
	}
	if o.lifeCycleAPIError != nil {

		fmt.Println("lifeCycleAPIError", o.lifeCycleAPIError)
	} else {
		fmt.Println("lifecycle", *o.lifecycle)
	}

}
