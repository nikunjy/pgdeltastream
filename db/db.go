package db

import (
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/nikunjy/pgdeltastream/logger"

	"github.com/jackc/pgx"
)

type Config struct {
	*pgx.ConnConfig
	SlotName string
}

// Initialize the database configuration
func (cfg *Config) WithDB(dbName, pgUser, pgPass, pgHost string, pgPort int) *Config {
	dbConfig := &pgx.ConnConfig{}
	dbConfig.Database = dbName
	dbConfig.Host = pgHost
	dbConfig.Port = uint16(pgPort)
	dbConfig.User = pgUser
	dbConfig.Password = pgPass
	cfg.ConnConfig = dbConfig
	return cfg
}

func NewSession(cfg *Config, log logger.Logger) (session *Session, retErr error) {
	// create a regular pg connection for use by transactions
	dbConfig := *cfg.ConnConfig
	pgConn, err := pgx.Connect(dbConfig)
	if err != nil {
		return nil, err
	}

	if log == nil {
		log = logger.NewDebugLogger()
	}

	defer func() {
		if retErr != nil {
			log.Error("Encountered error creating new session. Closing connection", retErr)
			pgConn.Close()
		}
	}()

	session = &Session{
		cfg: cfg,

		Logger:   log,
		PGConn:   pgConn,
		slotName: generateSlotName(cfg.SlotName),
	}

	log.Info("Creating replication connection to ", dbConfig.Database)

	replConn, err := pgx.ReplicationConnect(dbConfig)
	if err != nil {
		return nil, err
	}
	session.ReplConn = replConn

	defer func() {
		if retErr != nil {
			log.Error("Encountered error creating new session. Closing replication connection")
			replConn.Close()
		}
	}()

	log.Info("Creating replication slot ", session.slotName)
	var consistentPoint, snapshotName string
	if cfg.SlotName != "" {
		row := pgConn.QueryRow(
			`select plugin, restart_lsn from pg_replication_slots where slot_name=$1`,
			session.slotName,
		)
		var plugin string
		if err := row.Scan(&plugin, &consistentPoint); err != nil {
			return nil, err
		}
		if plugin != "wal2json" {
			return nil, errors.New("wrong plugin for the slot")
		}
	} else {
		consistentPoint, snapshotName, err = session.ReplConn.CreateReplicationSlotEx(session.slotName, "wal2json")

		if err != nil {
			return nil, err
		}
		log.Info("Created replication slot with consistent point", session.slotName, consistentPoint, snapshotName)
	}

	lsn, err := pgx.ParseLSN(consistentPoint)
	if err != nil {
		return nil, err
	}
	session.RestartLSN = lsn
	session.SnapshotName = snapshotName
	return session, nil
}

// CheckAndCreateReplConn creates a new replication connection
func (session *Session) ReCreateReplConn() error {
	if session.ReplConn != nil {
		if session.ReplConn.IsAlive() {
			// reuse the existing connection (or close it nonetheless?)
			return nil
		}
	}
	replConn, err := pgx.ReplicationConnect(*session.cfg.ConnConfig)
	if err != nil {
		return err
	}
	session.ReplConn = replConn
	return nil
}

// generates a random slot name which can be remembered
func generateSlotName(prefix string) string {
	// list of random words
	strs := []string{
		"gigantic",
		"scold",
		"greasy",
		"shaggy",
		"wasteful",
		"few",
		"face",
		"pet",
		"ablaze",
		"mundane",
	}
	rand.Seed(time.Now().Unix())
	if prefix == "" {
		prefix = strs[rand.Intn(len(strs))]
		return fmt.Sprintf("pgdelta_%s%d", prefix, rand.Intn(100))
	}
	// generate name such as delta_gigantic20
	return fmt.Sprintf("pgdelta_%s", prefix)
}

// delete all old slots that were created by us
func (session *Session) DeleteAllSlots() error {
	rows, err := session.PGConn.Query("SELECT slot_name FROM pg_replication_slots")
	if err != nil {
		return err
	}
	log := session.Logger
	for rows.Next() {
		var slotName string
		rows.Scan(&slotName)

		// only delete slots created by this program
		if !strings.Contains(slotName, "pgdelta_") {
			continue
		}

		log.Info("Deleting replication slot", slotName)
		err = session.ReplConn.DropReplicationSlot(slotName)
		if err != nil {
			log.Error("could not delete slot ", slotName, err)
		}
	}
	return nil
}
