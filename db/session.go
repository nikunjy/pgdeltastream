package db

import (
	"github.com/nikunjy/pgdeltastream/logger"
	"github.com/nikunjy/pgdeltastream/types"

	"github.com/jackc/pgx"
)

// Session stores the context, active db and ws connections, and replication slot state
type Session struct {
	cfg *Config

	ReplConn *pgx.ReplicationConn
	PGConn   *pgx.Conn

	Out chan types.Wal2JSONEvent

	slotName     string
	SnapshotName string
	RestartLSN   uint64
	Logger       logger.Logger
}
