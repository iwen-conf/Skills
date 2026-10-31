package main

// Catalog entries describe what the ledger holds; the static snapshot embeds
// them so the page never needs to query the ledger.

type ModelCatalogEntry struct {
	ID            string `json:"id"`
	Scope         string `json:"scope"`
	SchemaVersion int    `json:"schema_version"`
	System        string `json:"system"`
	Approved      bool   `json:"approved"`
}

type RoundCatalogEntry struct {
	ID          string `json:"id"`
	Graph       string `json:"graph"`
	Environment string `json:"environment"`
	Created     string `json:"created"`
	Validity    string `json:"validity"`
}

type RunnerCatalogEntry struct {
	ID       string `json:"id"`
	Graph    string `json:"graph"`
	Test     string `json:"test"`
	SHA256   string `json:"sha256"`
	Validity string `json:"validity"`
	Scope    string `json:"scope"`
}

type EnvironmentCatalogEntry struct {
	Name     string `json:"name"`
	Approved bool   `json:"approved"`
	Target   string `json:"target,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

type AccountCatalogEntry struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"`
	Environment string `json:"environment"`
	Role        string `json:"role,omitempty"`
	Scope       string `json:"scope,omitempty"`
}

type RunCatalogEntry struct {
	ID          string `json:"id"`
	Graph       string `json:"graph"`
	Task        string `json:"task"`
	Environment string `json:"environment"`
	Round       string `json:"round"`
	Status      string `json:"status"`
	Created     string `json:"created"`
}

type WorkbenchCatalog struct {
	Models           []ModelCatalogEntry       `json:"models"`
	Rounds           []RoundCatalogEntry       `json:"rounds"`
	Runners          []RunnerCatalogEntry      `json:"runners"`
	Fixtures         []AccountCatalogEntry     `json:"fixtures"`
	Runs             []RunCatalogEntry         `json:"runs"`
	Environments     []EnvironmentCatalogEntry `json:"environments"`
	EnvironmentError string                    `json:"environment_error,omitempty"`
	Truncated        bool                      `json:"truncated"`
}
