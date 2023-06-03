package s3

import "fmt"

func NoBucketFound() error {
	return fmt.Errorf("no bucket found")
}
func NoObjectFound(bucketname string) error {
	return fmt.Errorf("no object found for %s", bucketname)
}
func NoObjectFoundWithGivenPrefix(bucketname, prefix string) error {
	return fmt.Errorf("no object found for bucket %s with prefix %s", bucketname, prefix)
}
