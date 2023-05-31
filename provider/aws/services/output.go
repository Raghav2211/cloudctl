package services

import "cloudctl/viewer"

type ViewerFunc func(o interface{}) viewer.Viewer

type Fetcher interface {
	Fetch() (data interface{}, err error)
}

type Viewer interface {
	View(o interface{})
}
