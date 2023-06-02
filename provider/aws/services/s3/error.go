package s3

type NoBucketFound struct {
	error
}

func (err NoBucketFound) Error() string {
	return "No bucket found"
}
