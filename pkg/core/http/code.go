package http

// ErrorCode is a programatic representation of an error that occurred so
// that clients can distinguish between different errors without relying
// on free-form message text.
type ErrorCode string

// ErrorType is a distinguishes classes of errors so that clients can handle
// groups of errors in a particular way.
type ErrorType string

const (
	TypeUnknown            ErrorType = ""
	TypeValidationError    ErrorType = "validation_error"
	TypeUnderwritingError  ErrorType = "underwriting_error"
	TypePreconditionFailed ErrorType = "precondition_failed"
	TypeProcessingError    ErrorType = "processing_error"
)

const (
	CodeUnknown          ErrorCode = ""
	CodeUnsupportedGeo   ErrorCode = "unsupported_geo"
	CodeRiskRejected     ErrorCode = "risk_rejected"
	CodeAlreadySubmitted ErrorCode = "already_submitted"
	CodeSubmissionFailed ErrorCode = "submission_failed"
)
