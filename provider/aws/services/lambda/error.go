package lambda

import "fmt"

func NoFunctionFound() error {
	return fmt.Errorf("no function found")
}
