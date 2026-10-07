package main

import (
	"log"

	"mssh/internal/secrets"
	"mssh/internal/store"
)

// keychainMovedSetting marks the one-time move as done, in the settings table
// that already exists for exactly this kind of thing.
const keychainMovedSetting = "secrets.keychainMoved"

// moveKeychainPasswords hands passwords saved by versions before 0.6 from the
// OS keychain to the database, once.
//
// The moving itself is secrets.Move, which is tested. What is left here is
// the glue only main can provide: it is the one place that holds both vaults,
// and nothing else should ever know there were two.
func moveKeychainPasswords(st *store.Store, passwords secrets.Vault) {
	if done, _ := st.GetSetting(keychainMovedSetting); done == "1" {
		return
	}

	ids, err := allConnectionIDs(st)
	if err != nil {
		log.Printf("mssh: keychain move skipped: %v", err)
		return
	}

	if err := secrets.Move(ids, secrets.NewKeyring(), passwords); err != nil {
		// Not marked done, so the next launch tries again.
		log.Printf("mssh: keychain move stopped: %v", err)
		return
	}

	if err := st.SetSetting(keychainMovedSetting, "1"); err != nil {
		// Harmless: running the move again skips everything already moved.
		log.Printf("mssh: could not record the keychain move: %v", err)
	}
}

// allConnectionIDs lists every connection in every workspace.
func allConnectionIDs(st *store.Store) ([]string, error) {
	workspaces, err := st.ListWorkspaces()
	if err != nil {
		return nil, err
	}

	var ids []string
	for _, workspace := range workspaces {
		connections, err := st.ListConnections(workspace.ID)
		if err != nil {
			return nil, err
		}
		for _, connection := range connections {
			ids = append(ids, connection.ID)
		}
	}
	return ids, nil
}
