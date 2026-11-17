package main

// Campaign projections are mutable under a revision/fence. Events and dispatch
// intents are retained; a child run is preallocated and can be started once.
var businessCampaignSchema = []string{
	`CREATE TABLE IF NOT EXISTS business_campaigns (id TEXT PRIMARY KEY, graph TEXT NOT NULL, flow TEXT NOT NULL, environment TEXT NOT NULL, round TEXT NOT NULL, status TEXT NOT NULL, revision INTEGER NOT NULL, owner TEXT NOT NULL DEFAULT '', fence INTEGER NOT NULL DEFAULT 0, lease_until TEXT NOT NULL DEFAULT '', body_json TEXT NOT NULL, sha256 TEXT NOT NULL, created TEXT NOT NULL, updated TEXT NOT NULL);`,
	`CREATE INDEX IF NOT EXISTS idx_business_campaign_scope ON business_campaigns(graph,environment,round,created);`,
	`CREATE TABLE IF NOT EXISTS business_campaign_events (campaign_id TEXT NOT NULL, seq INTEGER NOT NULL, kind TEXT NOT NULL, previous_sha256 TEXT NOT NULL, sha256 TEXT NOT NULL, body_json TEXT NOT NULL, created TEXT NOT NULL, PRIMARY KEY(campaign_id,seq));`,
	`CREATE TABLE IF NOT EXISTS business_campaign_dispatches (campaign_id TEXT NOT NULL, seq INTEGER NOT NULL, run_id TEXT NOT NULL UNIQUE, variant_id TEXT NOT NULL, runner_id TEXT NOT NULL, task TEXT NOT NULL, status TEXT NOT NULL, intent_sha256 TEXT NOT NULL, body_json TEXT NOT NULL, created TEXT NOT NULL, updated TEXT NOT NULL, PRIMARY KEY(campaign_id,seq));`,
	`CREATE INDEX IF NOT EXISTS idx_business_campaign_child ON business_campaign_dispatches(run_id,status);`,
}
