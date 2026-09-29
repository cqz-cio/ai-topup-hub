package contract

import "errors"

var (
	ErrInvalid               = errors.New("coupon invalid")
	ErrNotFound              = errors.New("coupon not found")
	ErrInactive              = errors.New("coupon inactive")
	ErrNotStarted            = errors.New("coupon not started")
	ErrExpired               = errors.New("coupon expired")
	ErrUsageLimit            = errors.New("coupon usage limit")
	ErrPerUserLimit          = errors.New("coupon per user limit")
	ErrMinAmount             = errors.New("coupon min amount")
	ErrScopeInvalid          = errors.New("coupon scope invalid")
	ErrPaymentRoleNotAllowed = errors.New("coupon payment role not allowed")
	ErrPaymentRoleGuestOnly  = errors.New("coupon payment role guest only")
	ErrPaymentRoleMemberOnly = errors.New("coupon payment role member only")
	ErrMemberLevelNotAllowed = errors.New("coupon member level not allowed")
	ErrWholesaleDisabled     = errors.New("coupon wholesale disabled")
	ErrUpdateFailed          = errors.New("coupon update failed")
	ErrDeleteFailed          = errors.New("coupon delete failed")
)
