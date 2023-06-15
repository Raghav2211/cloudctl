package s3

import "fmt"

func NoBucketFound() error {
	return fmt.Errorf("no bucket found")
}
func NoObjectFound(bucketName string) error {
	return fmt.Errorf("no object found for %s", bucketName)
}
func NoObjectFoundWithGivenPrefix(bucketname, prefix string) error {
	return fmt.Errorf("no object found for bucket %s with prefix %s", bucketname, prefix)
}
func BucketContainMoreObject(bucketName string, maxKeys int64) error {
	return fmt.Errorf("%s contains more objects than the maximum key limit of %d", bucketName, maxKeys)
}
