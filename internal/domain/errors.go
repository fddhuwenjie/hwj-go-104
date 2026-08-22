package domain

import "errors"

// 领域错误类型：统一由 HTTP 层映射为 JSON 错误码。
var (
	ErrNotFound         = errors.New("resource not found")
	ErrConflict         = errors.New("optimistic lock conflict")
	ErrDuplicate        = errors.New("duplicate resource")
	ErrInvalidState     = errors.New("invalid state transition")
	ErrValidation       = errors.New("validation failed")
	ErrFrozen           = errors.New("route already frozen")
	ErrNotFrozen        = errors.New("route not frozen")
	ErrHoldBlocking     = errors.New("open hold blocks the lot")
	ErrStationSkip      = errors.New("illegal station skip")
	ErrQualification    = errors.New("equipment qualification does not cover run interval")
	ErrCapability       = errors.New("equipment or chamber capability mismatch")
	ErrRecipeFamily     = errors.New("recipe not compatible with equipment family")
	ErrWaferBusy        = errors.New("wafer already in a running process")
	ErrSampling         = errors.New("metrology sampling coverage insufficient")
	ErrRunNotCompleted  = errors.New("run not completed")
	ErrImmutable        = errors.New("immutable historical record")
	ErrIdempotency      = errors.New("idempotency key conflict with different payload")
)

// FieldError 描述一个字段级校验错误。
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ValidationError 携带一组字段错误。
type ValidationError struct {
	Fields []FieldError
}

func (e *ValidationError) Error() string { return ErrValidation.Error() }

// NewValidationError 构造字段校验错误。
func NewValidationError(fields ...FieldError) *ValidationError {
	return &ValidationError{Fields: fields}
}
