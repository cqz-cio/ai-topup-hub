package application

import "errors"

var (
	ErrNotFound               = errors.New("affiliate resource not found")
	ErrDisabled               = errors.New("affiliate disabled")
	ErrNotOpened              = errors.New("affiliate not opened")
	ErrCodeInvalid            = errors.New("affiliate code invalid")
	ErrProfileStatusInvalid   = errors.New("affiliate profile status invalid")
	ErrWithdrawAmountInvalid  = errors.New("affiliate withdraw amount invalid")
	ErrWithdrawChannelInvalid = errors.New("affiliate withdraw channel invalid")
	ErrWithdrawInsufficient   = errors.New("affiliate withdraw insufficient")
	ErrWithdrawStatusInvalid  = errors.New("affiliate withdraw status invalid")
	// ErrUserDisabled 开通推广时目标用户已禁用（与用户域共用同一文案哨兵）。
	ErrUserDisabled = errors.New("user disabled")
)
