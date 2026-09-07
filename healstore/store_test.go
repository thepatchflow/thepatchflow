package healstore

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRecordAndPersist(t *testing.T) {
	path := filepath.Join(t.TempDir(), "heals.jsonl")
	s := New(path)

	ev := s.Record(HealEvent{
		Vendor:   "stripe",
		Endpoint: "/v1/payments",
		Patch:    map[string]string{"charge": "amount"},
		Verified: true,
	})
	if ev.ID == "" {
		t.Fatal("expected generated ID")
	}

	s2 := New(path)
	got := s2.List(10)
	if len(got) != 1 {
		t.Fatalf("expected 1 event after reload, got %d", len(got))
	}
	if got[0].Endpoint != "/v1/payments" {
		t.Fatalf("unexpected endpoint: %s", got[0].Endpoint)
	}
}

func TestUpdatePRURL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "heals.jsonl")
	s := New(path)

	ev := s.Record(HealEvent{Vendor: "salesforce", Endpoint: "/cases"})
	s.UpdatePRURL(ev.ID, "https://github.com/x/y/pull/1")

	s2 := New(path)
	got := s2.List(10)[0]
	if got.PRURL != "https://github.com/x/y/pull/1" {
		t.Fatalf("PR URL not persisted: %q", got.PRURL)
	}
}

func TestLeaders(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "heals.jsonl"))
	s.Record(HealEvent{Vendor: "stripe", Endpoint: "/v1/payments", Verified: true})
	s.Record(HealEvent{Vendor: "stripe", Endpoint: "/v1/payments", Verified: false})
	s.Record(HealEvent{Vendor: "twilio", Endpoint: "/sms", Verified: true})

	leaders := s.Leaders()
	if len(leaders) != 2 {
		t.Fatalf("expected 2 vendors, got %d", len(leaders))
	}
	if leaders[0].Vendor != "stripe" || leaders[0].TotalHeals != 2 {
		t.Fatalf("stripe should lead with 2 heals, got %+v", leaders[0])
	}
}

func TestVendor(t *testing.T) {
	cases := map[string]string{
		"/v1/payments": "payments",
		"/v2/stripe/charges": "stripe",
		"":             "unknown",
	}
	for in, want := range cases {
		if got := Vendor(in); got != want {
			t.Errorf("Vendor(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDefaultPathParentExists(t *testing.T) {
	if os.MkdirAll(filepath.Dir(DefaultPath), 0o755) != nil {
		t.Fatal("failed to create default dir")
	}
	for _, p := range []string{"a"} {
		if DefaultPath == p {
			t.Fatal("bad default")
		}
	}
}

func TestPredictions(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "heals.jsonl"))
	s.Record(HealEvent{Vendor: "stripe", Endpoint: "/v1/charges", Patch: map[string]string{"card": "source"}})
	s.Record(HealEvent{Vendor: "stripe", Endpoint: "/v1/charges", Patch: map[string]string{"card": "source"}})
	s.Record(HealEvent{Vendor: "twilio", Endpoint: "/messages", Patch: map[string]string{"sid": "account_sid"}})

	preds := s.Predictions(0)
	if len(preds) != 2 {
		t.Fatalf("expected 2 predictions, got %d", len(preds))
	}
	// card->source seen twice should rank above sid->account_sid seen once
	if preds[0].OldKey != "card" || preds[0].NewKey != "source" {
		t.Fatalf("expected card->source to lead, got %+v", preds[0])
	}
	if preds[0].SeenCount != 2 {
		t.Fatalf("expected seen_count 2, got %d", preds[0].SeenCount)
	}
}

func TestSeedIdempotent(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "heals.jsonl"))
	first := s.Seed()
	second := s.Seed()
	if first != 8 {
		t.Fatalf("expected 8 seed records on first call, got %d", first)
	}
	if second != 0 {
		t.Fatalf("expected 0 new records on second call (idempotent), got %d", second)
	}
	if len(s.List(100)) != 8 {
		t.Fatalf("expected 8 total records, got %d", len(s.List(100)))
	}
}