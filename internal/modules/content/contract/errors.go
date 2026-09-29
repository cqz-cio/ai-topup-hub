package contract

import "errors"

var (
	ErrNotFound                      = errors.New("not found")
	ErrSlugExists                    = errors.New("slug exists")
	ErrCategoryParentInvalid         = errors.New("category parent invalid")
	ErrCategoryInUse                 = errors.New("category in use")
	ErrInvalidPostType               = errors.New("invalid post type")
	ErrPostCategoryInvalid           = errors.New("post category invalid")
	ErrPostNoticeCategoryUnsupported = errors.New("post notice category unsupported")
	ErrInvalidBanner                 = errors.New("invalid banner")
	ErrMediaNotFound                 = errors.New("media not found")
	ErrMediaNameEmpty                = errors.New("media name empty")
)
