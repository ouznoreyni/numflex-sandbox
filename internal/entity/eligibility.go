package entity

import "time"

const (
	// DelayBetweenPortings — délai entre deux portages, motif de rejet ARTP
	// « Dernier portage inférieur à 3 mois ».
	DelayBetweenPortings = 3 * 30 * 24 * time.Hour
	// DelayBeforeRestitution — délai avant restitution — §7.5 du guide.
	DelayBeforeRestitution = 6 * 30 * 24 * time.Hour
)

// NumberState is a number's current standing in the registry.
type NumberState struct {
	MSISDN            string
	CurrentOperatorID string
	OriginOperatorID  string
	LastPortingDate   *time.Time
	AlreadyRestituted bool
	RequestInProgress bool
}

// CheckPortingEligibility applies the portability eligibility rules.
func CheckPortingEligibility(n NumberState, sourceID, recipientID string,
	portingDelay time.Duration) *Fault {

	if n.CurrentOperatorID == recipientID {
		return NumberAlreadyAtRecipient()
	}
	if n.CurrentOperatorID != sourceID {
		return IncorrectSourceOperator()
	}
	if n.RequestInProgress {
		return RequestAlreadyInProgressForNumber()
	}
	if n.LastPortingDate != nil && time.Since(*n.LastPortingDate) < portingDelay {
		return PortingDelayNotRespected()
	}
	return nil
}

// CheckRestitutionEligibility applies the restitution eligibility rules.
func CheckRestitutionEligibility(n NumberState, restitutionDelay time.Duration) *Fault {
	if n.LastPortingDate == nil || n.CurrentOperatorID == n.OriginOperatorID {
		return NumberNotPorted()
	}
	if n.AlreadyRestituted {
		return NumberAlreadyRestituted()
	}
	if time.Since(*n.LastPortingDate) < restitutionDelay {
		return RestitutionDelayNotRespected()
	}
	if n.RequestInProgress {
		return RequestAlreadyInProgressForNumber()
	}
	return nil
}
