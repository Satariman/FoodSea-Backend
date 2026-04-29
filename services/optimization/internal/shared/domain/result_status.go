package domain

// ResultStatus is lifecycle status for optimization results.
type ResultStatus string

const (
	StatusActive  ResultStatus = "active"
	StatusLocked  ResultStatus = "locked"
	StatusExpired ResultStatus = "expired"
)
