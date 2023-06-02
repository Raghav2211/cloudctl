package aws

type APIException struct {
	error
}

func (err *APIException) Error() string {
	return "Somethig wrong, Need to find out what :)"
}
