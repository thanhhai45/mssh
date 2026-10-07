package secrets_test

import (
	"errors"
	"testing"

	"mssh/internal/secrets"
)

func TestMoveTakesEachSecretAcross(t *testing.T) {
	from, to := secrets.NewMemory(), secrets.NewMemory()
	_ = from.Set("a", "alpha")
	_ = from.Set("b", "beta")

	// "c" has no secret anywhere: that is not an error.
	if err := secrets.Move([]string{"a", "b", "c"}, from, to); err != nil {
		t.Fatalf("Move: %v", err)
	}

	for id, want := range map[string]string{"a": "alpha", "b": "beta"} {
		if got, err := to.Get(id); err != nil || got != want {
			t.Errorf("destination %s = %q, %v; want %q", id, got, err, want)
		}
		if from.Has(id) {
			t.Errorf("source still holds %s: two copies of one secret", id)
		}
	}
	if to.Has("c") {
		t.Error("an id with no secret gained one")
	}
}

func TestMoveNeverOverwritesTheDestination(t *testing.T) {
	from, to := secrets.NewMemory(), secrets.NewMemory()
	_ = from.Set("a", "old")
	_ = to.Set("a", "new")

	if err := secrets.Move([]string{"a"}, from, to); err != nil {
		t.Fatalf("Move: %v", err)
	}
	if got, _ := to.Get("a"); got != "new" {
		t.Errorf("destination = %q, want the newer %q", got, "new")
	}
	if from.Has("a") {
		t.Error("the stale source copy was left behind")
	}
}

// refusing is a vault whose writes always fail, standing in for a database
// that cannot be written to.
type refusing struct{ secrets.Vault }

func (refusing) Set(string, string) error { return errors.New("disk full") }

func TestMoveStopsWithoutLosingAnythingWhenTheDestinationFails(t *testing.T) {
	from := secrets.NewMemory()
	_ = from.Set("a", "alpha")
	to := refusing{secrets.NewMemory()}

	if err := secrets.Move([]string{"a"}, from, to); err == nil {
		t.Fatal("Move reported success although nothing could be written")
	}
	if got, _ := from.Get("a"); got != "alpha" {
		t.Error("the source copy was removed although the move failed")
	}
}
