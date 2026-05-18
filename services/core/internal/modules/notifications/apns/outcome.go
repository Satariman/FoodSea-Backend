package apns

import "github.com/sideshow/apns2"

type OutcomeClass string

const (
	OutcomeClassSuccess      OutcomeClass = "success"
	OutcomeClassInvalidToken OutcomeClass = "invalid_token"
	OutcomeClassTransient    OutcomeClass = "transient"
	OutcomeClassPermanent    OutcomeClass = "permanent"
)

// Outcome is a typed APNs delivery result for upper layers.
type Outcome struct {
	Sent       bool
	StatusCode int
	Reason     string
	ApnsID     string
	Class      OutcomeClass
}

// OutcomeFromResponse converts APNs response into transport-independent outcome.
func OutcomeFromResponse(response *apns2.Response) Outcome {
	if response == nil {
		return Outcome{
			Class: OutcomeClassPermanent,
		}
	}

	if response.Sent() {
		return Outcome{
			Sent:       true,
			StatusCode: response.StatusCode,
			Reason:     response.Reason,
			ApnsID:     response.ApnsID,
			Class:      OutcomeClassSuccess,
		}
	}

	return Outcome{
		Sent:       false,
		StatusCode: response.StatusCode,
		Reason:     response.Reason,
		ApnsID:     response.ApnsID,
		Class:      ClassifyReason(response.Reason),
	}
}

// ClassifyReason maps APNs reason code to caller-facing error class.
func ClassifyReason(reason string) OutcomeClass {
	switch reason {
	case apns2.ReasonUnregistered, apns2.ReasonBadDeviceToken, apns2.ReasonDeviceTokenNotForTopic:
		return OutcomeClassInvalidToken
	case apns2.ReasonTooManyRequests, apns2.ReasonInternalServerError:
		return OutcomeClassTransient
	default:
		return OutcomeClassPermanent
	}
}
