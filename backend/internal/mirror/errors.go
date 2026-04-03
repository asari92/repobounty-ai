package mirror

import "errors"

var (
	ErrMirrorNotFound        = errors.New("mirror not found")
	ErrCloneFailed           = errors.New("git clone failed")
	ErrFetchFailed           = errors.New("git fetch failed")
	ErrMetadataExtractFailed = errors.New("metadata extraction failed")
	ErrSyncTimeout           = errors.New("mirror sync timeout")
)
