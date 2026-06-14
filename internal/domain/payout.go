package domain

import "time"

// Payout records a settled cycle: the pooled contributions disbursed to the
// cycle's designated payee. It is the off-chain projection of an on-chain
// payout; DisbursementRef links it to the payment provider transaction.
type Payout struct {
	ID              string    `json:"id"`
	GroupID         string    `json:"group_id"`
	CycleIndex      int       `json:"cycle_index"`
	PayeeUserID     string    `json:"payee_user_id"`
	GrossAmount     Money     `json:"gross_amount"`     // sum collected
	ExpectedAmount  Money     `json:"expected_amount"`  // members * contribution
	Shortfall       Money     `json:"shortfall"`        // expected - gross (defaults)
	DisbursementRef string    `json:"disbursement_ref"` // payment provider ref
	SettledAt       time.Time `json:"settled_at"`
}
