package gopub

import "errors"

var (
	ErrNoContainerfile  = errors.New("epub: no containerfile found")
	ErrBadContainerfile = errors.New("epub: bad containerfile")
	ErrNoRootfile       = errors.New("epub: no rootfile found in container")
	ErrBadRootfile      = errors.New("epub: container references non-existent rootfile")
	ErrNoItemref        = errors.New("epub: no itemrefs found in spine")
	ErrBadItemref       = errors.New("epub: itemref references non-existent item")
	ErrBadManifest      = errors.New("epub: manifest references non-existent item")
	ErrMissingCoverId   = errors.New("epub: missing cover id in metadata")
	ErrFileTooLarge     = errors.New("epub: file exceeds MaxFileSize limit")
	ErrDuplicateID      = errors.New("epub: duplicate manifest item id")
)
