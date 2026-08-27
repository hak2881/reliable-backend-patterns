package ledger

import "sync"

type Event struct {
	IdempotencyKey string
	AccountID      string
	Amount         int64
	Reference      string
}

type Entry struct {
	IdempotencyKey string
	AccountID      string
	Amount         int64
	Reference      string
}

type OutboxEvent struct {
	Topic string
	Key   string
	Entry Entry
}

type Result struct {
	Recorded bool
	Balance  int64
}

type Tx interface {
	InsertEntry(Entry) (bool, error)
	InsertOutbox(OutboxEvent) error
	Balance(accountID string) (int64, error)
}

type Store interface {
	Transact(func(Tx) error) error
}

type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

func (s *Service) Record(event Event) (Result, error) {
	entry := Entry(event)
	var result Result

	err := s.store.Transact(func(tx Tx) error {
		inserted, err := tx.InsertEntry(entry)
		if err != nil {
			return err
		}
		if inserted {
			if err := tx.InsertOutbox(OutboxEvent{
				Topic: "ledger.entry_recorded",
				Key:   event.IdempotencyKey,
				Entry: entry,
			}); err != nil {
				return err
			}
		}

		balance, err := tx.Balance(event.AccountID)
		if err != nil {
			return err
		}
		result = Result{Recorded: inserted, Balance: balance}
		return nil
	})
	return result, err
}

type Snapshot struct {
	Entries []Entry
	Outbox  []OutboxEvent
}

type MemoryStore struct {
	mu      sync.Mutex
	entries []Entry
	outbox  []OutboxEvent
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{}
}

func (s *MemoryStore) Transact(fn func(Tx) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx := &memoryTx{
		entries: append([]Entry(nil), s.entries...),
		outbox:  append([]OutboxEvent(nil), s.outbox...),
	}
	if err := fn(tx); err != nil {
		return err
	}

	s.entries = tx.entries
	s.outbox = tx.outbox
	return nil
}

func (s *MemoryStore) Snapshot() Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()

	return Snapshot{
		Entries: append([]Entry(nil), s.entries...),
		Outbox:  append([]OutboxEvent(nil), s.outbox...),
	}
}

type memoryTx struct {
	entries []Entry
	outbox  []OutboxEvent
}

func (tx *memoryTx) InsertEntry(entry Entry) (bool, error) {
	for _, existing := range tx.entries {
		if existing.IdempotencyKey == entry.IdempotencyKey {
			return false, nil
		}
	}
	tx.entries = append(tx.entries, entry)
	return true, nil
}

func (tx *memoryTx) InsertOutbox(event OutboxEvent) error {
	tx.outbox = append(tx.outbox, event)
	return nil
}

func (tx *memoryTx) Balance(accountID string) (int64, error) {
	var balance int64
	for _, entry := range tx.entries {
		if entry.AccountID == accountID {
			balance += entry.Amount
		}
	}
	return balance, nil
}
