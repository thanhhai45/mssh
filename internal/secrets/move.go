package secrets

import "fmt"

func Move(ids []string, from Vault, to Vault) error {
	for _, id := range ids {
		if to.Has(id) {
			_ = from.Delete(id)
			continue
		}

		secret, err := from.Get(id)
		if err != nil {
			continue
		}

		if err := to.Set(id, secret); err != nil {
			return fmt.Errorf("move secret for %s: %w", id, err)
		}

		_ = from.Delete(id)
	}
	return nil
}
