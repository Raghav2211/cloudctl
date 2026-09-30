package changes

import "fmt"

func NoChangesFound() error {
	return fmt.Errorf("no changes found in the given window")
}
