package db

// OutboxTablesMySQL returns the MySQL DDL for the transactional outbox
// (RFC §8.1). The outbox table is written in the same SQL transaction as the
// business write so that no event can be lost between commit and publish.
func OutboxTablesMySQL() []string {
	return []string{
		`CREATE TABLE IF NOT EXISTS _kora_outbox (
			id VARCHAR(26) PRIMARY KEY,
			site VARCHAR(140) NOT NULL DEFAULT '',
			event_type VARCHAR(255) NOT NULL,
			event_version INT NOT NULL DEFAULT 1,
			aggregate_type VARCHAR(140) NOT NULL DEFAULT '',
			aggregate_id VARCHAR(255) NOT NULL DEFAULT '',
			payload JSON,
			status VARCHAR(20) NOT NULL DEFAULT 'pending',
			attempts INT NOT NULL DEFAULT 0,
			next_attempt_at DATETIME(6),
			lease_owner VARCHAR(140) NOT NULL DEFAULT '',
			lease_until DATETIME(6),
			created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
			published_at DATETIME(6),
			last_error TEXT,
			INDEX idx_outbox_pending (status, next_attempt_at),
			INDEX idx_outbox_site (site),
			INDEX idx_outbox_aggregate (aggregate_type, aggregate_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,

		`CREATE TABLE IF NOT EXISTS _kora_outbox_receipt (
			consumer_name VARCHAR(140) NOT NULL,
			event_id VARCHAR(255) NOT NULL,
			site VARCHAR(140) NOT NULL DEFAULT '',
			received_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
			PRIMARY KEY (consumer_name, event_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
	}
}

// OutboxTablesLibSQL returns the LibSQL-compatible outbox DDL.
func OutboxTablesLibSQL() []string {
	return []string{
		`CREATE TABLE IF NOT EXISTS _kora_outbox (
			id TEXT PRIMARY KEY,
			site TEXT NOT NULL DEFAULT '',
			event_type TEXT NOT NULL,
			event_version INTEGER NOT NULL DEFAULT 1,
			aggregate_type TEXT NOT NULL DEFAULT '',
			aggregate_id TEXT NOT NULL DEFAULT '',
			payload TEXT,
			status TEXT NOT NULL DEFAULT 'pending',
			attempts INTEGER NOT NULL DEFAULT 0,
			next_attempt_at TEXT,
			lease_owner TEXT NOT NULL DEFAULT '',
			lease_until TEXT,
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			published_at TEXT,
			last_error TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_outbox_pending ON _kora_outbox (status, next_attempt_at)`,
		`CREATE INDEX IF NOT EXISTS idx_outbox_site ON _kora_outbox (site)`,
		`CREATE INDEX IF NOT EXISTS idx_outbox_aggregate ON _kora_outbox (aggregate_type, aggregate_id)`,

		`CREATE TABLE IF NOT EXISTS _kora_outbox_receipt (
			consumer_name TEXT NOT NULL,
			event_id TEXT NOT NULL,
			site TEXT NOT NULL DEFAULT '',
			received_at TEXT NOT NULL DEFAULT (datetime('now')),
			PRIMARY KEY (consumer_name, event_id)
		)`,
	}
}

// OutboxTablesPostgres returns the PostgreSQL-compatible outbox DDL.
func OutboxTablesPostgres() []string {
	return []string{
		`CREATE TABLE IF NOT EXISTS "_kora_outbox" (
			"id" VARCHAR(26) PRIMARY KEY,
			"site" VARCHAR(140) NOT NULL DEFAULT '',
			"event_type" VARCHAR(255) NOT NULL,
			"event_version" INTEGER NOT NULL DEFAULT 1,
			"aggregate_type" VARCHAR(140) NOT NULL DEFAULT '',
			"aggregate_id" VARCHAR(255) NOT NULL DEFAULT '',
			"payload" JSONB,
			"status" VARCHAR(20) NOT NULL DEFAULT 'pending',
			"attempts" INTEGER NOT NULL DEFAULT 0,
			"next_attempt_at" TIMESTAMP,
			"lease_owner" VARCHAR(140) NOT NULL DEFAULT '',
			"lease_until" TIMESTAMP,
			"created_at" TIMESTAMP NOT NULL DEFAULT NOW(),
			"published_at" TIMESTAMP,
			"last_error" TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_outbox_pending ON "_kora_outbox" ("status", "next_attempt_at")`,
		`CREATE INDEX IF NOT EXISTS idx_outbox_site ON "_kora_outbox" ("site")`,
		`CREATE INDEX IF NOT EXISTS idx_outbox_aggregate ON "_kora_outbox" ("aggregate_type", "aggregate_id")`,

		`CREATE TABLE IF NOT EXISTS "_kora_outbox_receipt" (
			"consumer_name" VARCHAR(140) NOT NULL,
			"event_id" VARCHAR(255) NOT NULL,
			"site" VARCHAR(140) NOT NULL DEFAULT '',
			"received_at" TIMESTAMP NOT NULL DEFAULT NOW(),
			PRIMARY KEY ("consumer_name", "event_id")
		)`,
	}
}
