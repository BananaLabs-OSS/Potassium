package wallet

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	_ "modernc.org/sqlite"
)

// setupTestDB returns an in-memory SQLite + the wallet schema created.
// Each test gets its own DB so assertions don't leak across tests.
func setupTestDB(t *testing.T) *bun.DB {
	t.Helper()
	sqldb, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { sqldb.Close() })
	db := bun.NewDB(sqldb, sqlitedialect.New())
	ctx := context.Background()
	if _, err := db.NewCreateTable().Model((*Wallet)(nil)).Exec(ctx); err != nil {
		t.Fatalf("create wallets table: %v", err)
	}
	if _, err := db.NewCreateTable().Model((*Transaction)(nil)).Exec(ctx); err != nil {
		t.Fatalf("create wallet_transactions table: %v", err)
	}
	return db
}

func TestNormalizeEmail(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"SirNiklas9@proton.me", "sirniklas9@proton.me"},
		{"  Padded@Example.COM  ", "padded@example.com"},
		{"already@lower.case", "already@lower.case"},
		{"", ""},
		{"   ", ""},
	}
	for _, c := range cases {
		got := NormalizeEmail(c.in)
		if got != c.want {
			t.Errorf("NormalizeEmail(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestCreditCreatesWalletAndAuditRow(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	if err := Credit(ctx, db, "alice@example.com", 500, TxTopup, "pi_test1", "stripe_pi", "topup"); err != nil {
		t.Fatalf("Credit: %v", err)
	}

	bal, err := GetBalance(ctx, db, "alice@example.com")
	if err != nil || bal != 500 {
		t.Fatalf("GetBalance after credit: bal=%d err=%v", bal, err)
	}

	var w Wallet
	if err := db.NewSelect().Model(&w).Where("email = ?", "alice@example.com").Scan(ctx); err != nil {
		t.Fatalf("wallet row missing: %v", err)
	}
	if w.LifetimeCreditedCents != 500 {
		t.Errorf("LifetimeCreditedCents = %d, want 500", w.LifetimeCreditedCents)
	}

	count, _ := db.NewSelect().Model((*Transaction)(nil)).Where("email = ?", "alice@example.com").Count(ctx)
	if count != 1 {
		t.Errorf("transaction count = %d, want 1", count)
	}
}

func TestChargeSucceedsThenInsufficient(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	if err := Credit(ctx, db, "bob@example.com", 1000, TxTopup, "pi_test2", "stripe_pi", ""); err != nil {
		t.Fatalf("seed credit: %v", err)
	}
	if err := Charge(ctx, db, "bob@example.com", 400, TxCheckoutApplied, "order_1", "order", "checkout"); err != nil {
		t.Fatalf("first charge: %v", err)
	}
	bal, _ := GetBalance(ctx, db, "bob@example.com")
	if bal != 600 {
		t.Errorf("balance after charge = %d, want 600", bal)
	}

	if err := Charge(ctx, db, "bob@example.com", 700, TxCheckoutApplied, "order_2", "order", ""); !errors.Is(err, ErrInsufficientBalance) {
		t.Errorf("over-charge err = %v, want ErrInsufficientBalance", err)
	}
	balAfter, _ := GetBalance(ctx, db, "bob@example.com")
	if balAfter != 600 {
		t.Errorf("balance after failed charge = %d, want 600 (unchanged)", balAfter)
	}
}

func TestChargeAgainstNonexistentWalletIsInsufficient(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	err := Charge(ctx, db, "nobody@example.com", 100, TxCheckoutApplied, "order_x", "order", "")
	if !errors.Is(err, ErrInsufficientBalance) {
		t.Errorf("err = %v, want ErrInsufficientBalance", err)
	}
}

func TestCaseInsensitiveAcrossOperations(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Credit with mixed case → row stored lowercase.
	if err := Credit(ctx, db, "Carol@Example.COM", 200, TxTopup, "pi_test3", "stripe_pi", ""); err != nil {
		t.Fatalf("Credit: %v", err)
	}
	// Read with uppercase resolves to the same row.
	bal, _ := GetBalance(ctx, db, "CAROL@EXAMPLE.COM")
	if bal != 200 {
		t.Errorf("GetBalance uppercase = %d, want 200", bal)
	}
	// Charge with original mixed case debits the same row.
	if err := Charge(ctx, db, "Carol@Example.COM", 50, TxCheckoutApplied, "order_3", "order", ""); err != nil {
		t.Fatalf("Charge: %v", err)
	}
	bal, _ = GetBalance(ctx, db, "carol@example.com")
	if bal != 150 {
		t.Errorf("GetBalance after cross-case charge = %d, want 150", bal)
	}

	// Only ONE wallet row exists (no case-duplicates).
	count, _ := db.NewSelect().Model((*Wallet)(nil)).Count(ctx)
	if count != 1 {
		t.Errorf("wallet rows after cross-case ops = %d, want 1", count)
	}
}

func TestGetBalanceMissingEmailReturnsZero(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	bal, err := GetBalance(ctx, db, "ghost@example.com")
	if err != nil {
		t.Errorf("err = %v, want nil for missing wallet", err)
	}
	if bal != 0 {
		t.Errorf("bal = %d, want 0", bal)
	}
}

func TestEnsureExistsIsIdempotent(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		if err := EnsureExists(ctx, db, "dave@example.com"); err != nil {
			t.Fatalf("EnsureExists iter %d: %v", i, err)
		}
	}
	count, _ := db.NewSelect().Model((*Wallet)(nil)).Count(ctx)
	if count != 1 {
		t.Errorf("wallet count after 3 EnsureExists = %d, want 1", count)
	}
	bal, _ := GetBalance(ctx, db, "dave@example.com")
	if bal != 0 {
		t.Errorf("seeded balance = %d, want 0", bal)
	}
}

func TestInvalidAmountsRejected(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	if err := Charge(ctx, db, "eve@example.com", 0, TxCheckoutApplied, "", "", ""); err == nil {
		t.Error("Charge with amount=0: expected error")
	}
	if err := Charge(ctx, db, "eve@example.com", -10, TxCheckoutApplied, "", "", ""); err == nil {
		t.Error("Charge with amount=-10: expected error")
	}
	if err := Credit(ctx, db, "eve@example.com", 0, TxTopup, "", "", ""); err == nil {
		t.Error("Credit with amount=0: expected error")
	}
	if err := Credit(ctx, db, "eve@example.com", -10, TxTopup, "", "", ""); err == nil {
		t.Error("Credit with amount=-10: expected error")
	}
}

func TestEmptyEmailRejected(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	if err := Charge(ctx, db, "", 100, TxCheckoutApplied, "", "", ""); err == nil {
		t.Error("Charge with empty email: expected error")
	}
	if err := Credit(ctx, db, "   ", 100, TxTopup, "", "", ""); err == nil {
		t.Error("Credit with whitespace-only email: expected error")
	}
	if err := EnsureExists(ctx, db, ""); err == nil {
		t.Error("EnsureExists with empty email: expected error")
	}
}

func TestAuditRowSignAndDescription(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	if err := Credit(ctx, db, "frank@example.com", 1000, TxTopup, "pi_f", "stripe_pi", "topup"); err != nil {
		t.Fatalf("Credit: %v", err)
	}
	if err := Charge(ctx, db, "frank@example.com", 250, TxCheckoutApplied, "order_f", "order", "checkout"); err != nil {
		t.Fatalf("Charge: %v", err)
	}

	var txs []Transaction
	if err := db.NewSelect().Model(&txs).Where("email = ?", "frank@example.com").OrderExpr("created_at ASC").Scan(ctx); err != nil {
		t.Fatalf("list transactions: %v", err)
	}
	if len(txs) != 2 {
		t.Fatalf("transaction count = %d, want 2", len(txs))
	}
	if txs[0].AmountCents != 1000 || txs[0].Type != TxTopup {
		t.Errorf("credit row: amount=%d type=%s, want 1000 topup", txs[0].AmountCents, txs[0].Type)
	}
	if txs[1].AmountCents != -250 || txs[1].Type != TxCheckoutApplied {
		t.Errorf("debit row: amount=%d type=%s, want -250 checkout_applied", txs[1].AmountCents, txs[1].Type)
	}
}
