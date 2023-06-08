package fetcher

type Fetcher interface {
	Fetch() (data interface{})
}
