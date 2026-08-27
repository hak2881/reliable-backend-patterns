package ledger

import (
	"errors"
	"testing"
)

func TestRecordCreatesEntryAndOutbox(t *testing.T) {
	store := NewMemoryStore()
	service := NewService(store)

	got, err := service.Record(Event{
		IdempotencyKey: "order-paid:1001",
		AccountID:      "customer-42",
		Amount:         1250,
		Reference:      "order-1001",
	})
	if err != nil {
		t.Fatalf("Record() error = %v", err)
	}

	want := Result{Recorded: true, Balance: 1250}
	if got != want {
		t.Fatalf("Record() = %+v, want %+v", got, want)
	}

	snapshot := store.Snapshot()
	if len(snapshot.Entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(snapshot.Entries))
	}
	if len(snapshot.Outbox) != 1 {
		t.Fatalf("outbox events = %d, want 1", len(snapshot.Outbox))
	}
	if snapshot.Outbox[0].Entry != snapshot.Entries[0] {
		t.Fatalf("outbox entry = %+v, ledger entry = %+v", snapshot.Outbox[0].Entry, snapshot.Entries[0])
	}
}

func TestRecordTreatsDuplicateIdempotencyKeyAsNoOp(t *testing.T) {
	store := NewMemoryStore()
	service := NewService(store)
	event := Event{
		IdempotencyKey: "order-paid:1001",
		AccountID:      "customer-42",
		Amount:         1250,
		Reference:      "order-1001",
	}

	if _, err := service.Record(event); err != nil {
		t.Fatalf("first Record() error = %v", err)
	}
	got, err := service.Record(event)
	if err != nil {
		t.Fatalf("duplicate Record() error = %v", err)
	}

	want := Result{Recorded: false, Balance: 1250}
	if got != want {
		t.Fatalf("duplicate Record() = %+v, want %+v", got, want)
	}

	snapshot := store.Snapshot()
	if len(snapshot.Entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(snapshot.Entries))
	}
	if len(snapshot.Outbox) != 1 {
		t.Fatalf("outbox events = %d, want 1", len(snapshot.Outbox))
	}
}

func TestBalanceUsesSignedIntegerEntries(t *testing.T) {
	store := NewMemoryStore()
	service := NewService(store)

	entries := []Event{
		{IdempotencyKey: "order-paid:1001", AccountID: "customer-42", Amount: 1250, Reference: "order-1001"},
		{IdempotencyKey: "order-refunded:1001", AccountID: "customer-42", Amount: -250, Reference: "refund-1001"},
	}

	var got Result
	for _, event := range entries {
		var err error
		got, err = service.Record(event)
		if err != nil {
			t.Fatalf("Record(%q) error = %v", event.IdempotencyKey, err)
		}
	}

	if got.Balance != 1000 {
		t.Fatalf("balance = %d, want 1000", got.Balance)
	}
}

func TestTransactionRollsBackWhenOutboxStepFails(t *testing.T) {
	store := NewMemoryStore()
	outboxErr := errors.New("outbox unavailable")

	err := store.Transact(func(tx Tx) error {
		inserted, err := tx.InsertEntry(Entry{
			IdempotencyKey: "order-paid:1001",
			AccountID:      "customer-42",
			Amount:         1250,
			Reference:      "order-1001",
		})
		if err != nil {
			return err
		}
		if !inserted {
			t.Fatal("first entry was treated as a duplicate")
		}
		return outboxErr
	})
	if !errors.Is(err, outboxErr) {
		t.Fatalf("Transact() error = %v, want %v", err, outboxErr)
	}

	snapshot := store.Snapshot()
	if len(snapshot.Entries) != 0 {
		t.Fatalf("entries after rollback = %d, want 0", len(snapshot.Entries))
	}
	if len(snapshot.Outbox) != 0 {
		t.Fatalf("outbox events after rollback = %d, want 0", len(snapshot.Outbox))
	}
}
