package store

import (
	"errors"
	"testing"
)

func TestSettingRoundTrip(t *testing.T) {
	s := openTest(t)

	if _, err := s.GetSetting("terminal.fontSize"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unset key: err = %v, want ErrNotFound", err)
	}

	if err := s.SetSetting("terminal.fontSize", "13"); err != nil {
		t.Fatalf("SetSetting: %v", err)
	}

	got, err := s.GetSetting("terminal.fontSize")
	if err != nil {
		t.Fatalf("GetSetting: %v", err)
	}
	if got != "13" {
		t.Errorf("value = %q, want %q", got, "13")
	}
}

// TestSetSettingReplaces is the whole point of the UPSERT: writing twice is
// the normal case, not an error.
func TestSetSettingReplaces(t *testing.T) {
	s := openTest(t)

	if err := s.SetSetting("ui.theme", "dark"); err != nil {
		t.Fatalf("first SetSetting: %v", err)
	}
	if err := s.SetSetting("ui.theme", "light"); err != nil {
		t.Fatalf("second SetSetting: %v", err)
	}

	got, err := s.GetSetting("ui.theme")
	if err != nil {
		t.Fatalf("GetSetting: %v", err)
	}
	if got != "light" {
		t.Errorf("value = %q, want the second write to win", got)
	}
}

func TestGetAllSettings(t *testing.T) {
	s := openTest(t)

	all, err := s.GetAllSettings()
	if err != nil {
		t.Fatalf("GetAllSettings on an empty table: %v", err)
	}
	if all == nil {
		t.Fatal("returned a nil map; the frontend would receive JSON null")
	}
	if len(all) != 0 {
		t.Errorf("got %d settings, want none", len(all))
	}

	for key, value := range map[string]string{
		"ui.theme":          "dark",
		"terminal.fontSize": "13",
		"terminal.theme":    "dracula",
	} {
		if err := s.SetSetting(key, value); err != nil {
			t.Fatalf("SetSetting %s: %v", key, err)
		}
	}

	all, err = s.GetAllSettings()
	if err != nil {
		t.Fatalf("GetAllSettings: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("got %d settings, want 3", len(all))
	}
	if all["terminal.theme"] != "dracula" {
		t.Errorf("terminal.theme = %q, want dracula", all["terminal.theme"])
	}
}

func TestDeleteSettingIsIdempotent(t *testing.T) {
	s := openTest(t)

	if err := s.SetSetting("ui.accent", "violet"); err != nil {
		t.Fatalf("SetSetting: %v", err)
	}
	if err := s.DeleteSetting("ui.accent"); err != nil {
		t.Fatalf("DeleteSetting: %v", err)
	}
	if _, err := s.GetSetting("ui.accent"); !errors.Is(err, ErrNotFound) {
		t.Errorf("after delete: err = %v, want ErrNotFound", err)
	}

	// Deleting again is the caller getting what they asked for.
	if err := s.DeleteSetting("ui.accent"); err != nil {
		t.Errorf("second DeleteSetting: %v", err)
	}
}

func TestSettingRejectsBlankKey(t *testing.T) {
	s := openTest(t)

	if err := s.SetSetting("   ", "x"); err == nil {
		t.Error("blank key was accepted")
	}
	if _, err := s.GetSetting(""); err == nil {
		t.Error("blank key was accepted by GetSetting")
	}
}
