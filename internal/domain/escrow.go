package domain

import "time"

// LedgerDirection is the sign of an escrow ledger entry.
type LedgerDirection string

const (
	// LedgerCredit increases a group's escrow balance (contribution collected).
	LedgerCredit LedgerDirection = "credit"
	// LedgerDebit decreases it (payout disbursed).
	LedgerDebit LedgerDirection = "debit"
)

// EscrowEntry is one immutable line in a group's escrow ledger. The escrow
// account holds collected contributions until a cycle settles and they are
// disbursed to the payee. Like reputation, the ledger is append-only.
type EscrowEntry struct {
	ID         string          `json:"id"`
	GroupID    string          `json:"group_id"`
	CycleIndex int             `json:"cycle_index"`
	UserID     string          `json:"user_id"` // empty for group-level debits
	Direction  LedgerDirection `json:"direction"`
	Amount     Money           `json:"amount"`
	Reference  string          `json:"reference"` // payment provider tx ref
	Memo       string          `json:"memo"`
	CreatedAt  time.Time       `json:"created_at"`
}

// EscrowBalance folds a ledger into a current balance for a group.
func EscrowBalance(entries []*EscrowEntry) Money {
	var bal Money
	for _, e := range entries {
		switch e.Direction {
		case LedgerCredit:
			bal += e.Amount
		case LedgerDebit:
			bal -= e.Amount
		}
	}
	return bal
}
