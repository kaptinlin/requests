package requests

import "fmt"

// Profile applies a coherent client identity to a Client.
type Profile interface {
	Name() string
	Apply(*Client) error
}

// ApplyProfile applies profile to the client.
func (c *Client) ApplyProfile(profile Profile) error {
	if c == nil {
		return fmt.Errorf("%w: client", ErrInvalidConfigValue)
	}
	if profile == nil {
		return fmt.Errorf("%w: profile", ErrInvalidConfigValue)
	}
	if err := profile.Apply(c); err != nil {
		return fmt.Errorf("apply profile %q: %w", profile.Name(), err)
	}
	return nil
}
