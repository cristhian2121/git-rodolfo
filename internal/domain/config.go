package domain

// CurrentConfigVersion is the version written to new config files.
const CurrentConfigVersion = 1

// Settings holds UI-only preferences that never affect which account
// applies to a repository (PRD §11.2, §17).
type Settings struct {
	LastUsedAccountID *string `json:"lastUsedAccountId"`
}

// GlobalConfig is the root of ~/.config/git-rodolfo/config.json (PRD §17, §18).
type GlobalConfig struct {
	Version  int       `json:"version"`
	Settings Settings  `json:"settings"`
	Accounts []Account `json:"accounts"`
}

// NewGlobalConfig returns an empty, valid config for a fresh install.
func NewGlobalConfig() GlobalConfig {
	return GlobalConfig{
		Version:  CurrentConfigVersion,
		Settings: Settings{},
		Accounts: []Account{},
	}
}
