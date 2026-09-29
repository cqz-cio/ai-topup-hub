package contract

import "errors"

var (
	ErrExists       = errors.New("api credential already exists for this user")
	ErrNotFound     = errors.New("api credential not found")
	ErrNotApproved  = errors.New("api credential is not approved")
	ErrPendingExist = errors.New("pending application already exists")
)
