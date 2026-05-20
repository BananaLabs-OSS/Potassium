// Package wallet provides atomic credit/debit primitives over a SQLite
// `wallets` + `wallet_transactions` schema. Multiple cells/services in
// the BananaKit ecosystem share the same wallet semantics; this package
// is the single source of truth so the primitives can't drift.
//
// Schema (created by each consumer's migrations; shapes must match):
//
//	wallets(email TEXT PK, wallet_cents INTEGER, lifetime_credited_cents INTEGER, updated_at TIMESTAMP)
//	wallet_transactions(id TEXT PK, email TEXT, amount_cents INTEGER, type TEXT, reference_id TEXT, reference_type TEXT, description TEXT, created_at TIMESTAMP)
//
// Every mutation is a single SQLite transaction that updates the wallet
// row AND writes a wallet_transactions audit row. SQLite single-writer
// mode means BEGIN serializes against other writers, so the read-modify-
// write loop is safe without an optimistic-lock version column.
package wallet

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

// Wallet is the per-email balance row. Email is the primary key.
type Wallet struct {
	bun.BaseModel `bun:"table:wallets,alias:w"`

	Email                 string    `bun:"email,pk,type:text"                        json:"email"`
	WalletCents           int       `bun:"wallet_cents,notnull,default:0"            json:"wallet_cents"`
	LifetimeCreditedCents int       `bun:"lifetime_credited_cents,notnull,default:0" json:"lifetime_credited_cents"`
	UpdatedAt             time.Time `bun:"updated_at,nullzero,notnull"               json:"updated_at"`
}

// Transaction is one credit or debit on a wallet. AmountCents is negative
// for debits, positive for credits. Type is a free string at the DB
// level but funneling callers through the named consts below keeps the
// audit log searchable. ReferenceID + ReferenceType point at the origin
// (order id, PI id, pool id, …) but there are no FKs at the DB level —
// the audit log keeps writing even if the source row is later deleted.
type Transaction struct {
	bun.BaseModel `bun:"table:wallet_transactions,alias:wt"`

	ID            string    `bun:"id,pk,type:text"             json:"id"`
	Email         string    `bun:"email,notnull,type:text"     json:"email"`
	AmountCents   int       `bun:"amount_cents,notnull"        json:"amount_cents"`
	Type          string    `bun:"type,notnull,type:text"      json:"type"`
	ReferenceID   string    `bun:"reference_id,type:text"      json:"reference_id,omitempty"`
	ReferenceType string    `bun:"reference_type,type:text"    json:"reference_type,omitempty"`
	Description   string    `bun:"description,type:text"       json:"description,omitempty"`
	CreatedAt     time.Time `bun:"created_at,nullzero,notnull" json:"created_at"`
}

// ErrInsufficientBalance is returned by Charge when the debit would
// push wallet_cents below zero (or when the wallet row doesn't exist —
// you can't charge from nothing).
var ErrInsufficientBalance = errors.New("wallet: insufficient balance")

// Generic transaction type constants — events any wallet-using product
// will emit. Product-specific event types (e.g. session_deploy,
// stripe_decline_reversal) stay defined locally in the consumer to
// keep this package's vocabulary product-agnostic.
const (
	TxCheckoutApplied = "checkout_applied" // debit: wallet credit applied at purchase
	TxProrateCharge   = "prorate_charge"   // debit: upgrade / reconfigure top-up
	TxRefund          = "refund"           // credit: order refunded back to wallet
	TxGiftClaim       = "gift_claim"       // credit: gift voucher claimed
	TxPoolPayout      = "pool_payout"      // credit: pool settled, refund to participants
	TxPromoCredit     = "promo_credit"     // credit: promo / outreach credit
	TxProrateRefund   = "prorate_refund"   // credit: downgrade / reconfigure rebate
	TxOutageCredit    = "outage_credit"    // credit: SLA / goodwill credit
	TxAdminAdjustment = "admin_adjustment" // either direction: admin-driven correction
	TxTopup           = "topup"            // credit: customer Stripe top-up; ReferenceID = PI.ID for idempotency
)

// NormalizeEmail lowercases and trims the wallet key. Every write path
// in every consumer normalizes before touching the row so a single
// mixed-case row can't surface as "balance reads work but charge fails"
// for the same customer. Exposed publicly so consumers that look up
// emails for OTHER purposes (e.g. ban checks against the user_bans
// table) can use the same canonical form without duplicating the
// helper.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// Charge debits the wallet atomically. Returns ErrInsufficientBalance
// if the wallet doesn't exist OR if the resulting balance would go
// negative. Both the wallet row update and the wallet_transactions
// insert happen in the same SQLite transaction; either both land or
// neither does.
//
// amountCents must be positive (the magnitude to debit). The audit
// row's AmountCents is stored as the negative of this value.
func Charge(ctx context.Context, db *bun.DB, email string, amountCents int, txType, refID, refType, description string) error {
	if amountCents <= 0 {
		return errors.New("wallet: charge amount must be positive")
	}
	email = NormalizeEmail(email)
	if email == "" {
		return errors.New("wallet: email required")
	}

	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		var w Wallet
		err := tx.NewSelect().Model(&w).Where("email = ?", email).Scan(ctx)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrInsufficientBalance
		}
		if err != nil {
			return err
		}

		if w.WalletCents < amountCents {
			return ErrInsufficientBalance
		}

		now := time.Now().UTC()
		newBalance := w.WalletCents - amountCents

		// Guarded WHERE on the old balance — single-writer makes the
		// race impossible, but the guard catches programmer error
		// where a caller shares a tx across goroutines.
		res, err := tx.NewUpdate().Model((*Wallet)(nil)).
			Set("wallet_cents = ?", newBalance).
			Set("updated_at = ?", now).
			Where("email = ?", email).
			Where("wallet_cents = ?", w.WalletCents).
			Exec(ctx)
		if err != nil {
			return err
		}
		affected, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if affected != 1 {
			return ErrInsufficientBalance
		}

		auditRow := &Transaction{
			ID:            uuid.New().String(),
			Email:         email,
			AmountCents:   -amountCents,
			Type:          txType,
			ReferenceID:   refID,
			ReferenceType: refType,
			Description:   description,
			CreatedAt:     now,
		}
		if _, err := tx.NewInsert().Model(auditRow).Exec(ctx); err != nil {
			return err
		}
		return nil
	})
}

// Credit credits the wallet atomically. Creates the wallet row lazily
// if it doesn't exist (first-credit semantics). Both the wallet upsert
// and the audit row insert happen in the same transaction.
//
// amountCents must be positive (the magnitude to credit). The audit
// row's AmountCents is stored as the positive value.
func Credit(ctx context.Context, db *bun.DB, email string, amountCents int, txType, refID, refType, description string) error {
	if amountCents <= 0 {
		return errors.New("wallet: credit amount must be positive")
	}
	email = NormalizeEmail(email)
	if email == "" {
		return errors.New("wallet: email required")
	}

	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		now := time.Now().UTC()

		seed := &Wallet{
			Email:                 email,
			WalletCents:           0,
			LifetimeCreditedCents: 0,
			UpdatedAt:             now,
		}
		if _, err := tx.NewInsert().Model(seed).Ignore().Exec(ctx); err != nil {
			return err
		}

		if _, err := tx.NewUpdate().Model((*Wallet)(nil)).
			Set("wallet_cents = wallet_cents + ?", amountCents).
			Set("lifetime_credited_cents = lifetime_credited_cents + ?", amountCents).
			Set("updated_at = ?", now).
			Where("email = ?", email).
			Exec(ctx); err != nil {
			return err
		}

		auditRow := &Transaction{
			ID:            uuid.New().String(),
			Email:         email,
			AmountCents:   amountCents,
			Type:          txType,
			ReferenceID:   refID,
			ReferenceType: refType,
			Description:   description,
			CreatedAt:     now,
		}
		if _, err := tx.NewInsert().Model(auditRow).Exec(ctx); err != nil {
			return err
		}
		return nil
	})
}

// GetBalance returns the spendable balance for email. Returns (0, nil)
// when no wallet row exists — that's a valid state (the user has just
// never received a credit).
func GetBalance(ctx context.Context, db *bun.DB, email string) (int, error) {
	email = NormalizeEmail(email)
	if email == "" {
		return 0, errors.New("wallet: email required")
	}
	var w Wallet
	err := db.NewSelect().Model(&w).Column("wallet_cents").Where("email = ?", email).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return w.WalletCents, nil
}

// EnsureExists creates an empty wallet row for email if none exists.
// Idempotent — safe to call repeatedly. Useful at registration time
// so admin tooling doesn't show "no wallet" for users who've never
// received a credit but have placed orders.
func EnsureExists(ctx context.Context, db *bun.DB, email string) error {
	email = NormalizeEmail(email)
	if email == "" {
		return errors.New("wallet: email required")
	}
	seed := &Wallet{
		Email:                 email,
		WalletCents:           0,
		LifetimeCreditedCents: 0,
		UpdatedAt:             time.Now().UTC(),
	}
	_, err := db.NewInsert().Model(seed).Ignore().Exec(ctx)
	return err
}
