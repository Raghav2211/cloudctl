package s3

import "fmt"

func NoBucketFound() error {
	return fmt.Errorf("no bucket found")
}
