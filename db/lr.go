package db

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx"
	"github.com/nikunjy/pgdeltastream/types"
	log "github.com/sirupsen/logrus"
)

var statusHeartbeatIntervalSeconds = 10

// LRStream will start streaming changes from the given slotName over the websocket connection
func (session *Session) LRStream(ctx context.Context) error {
	log := session.Logger
	log.Info("Starting replication for slot", session.slotName, pgx.FormatLSN(session.RestartLSN))
	err := session.ReplConn.StartReplication(session.slotName, session.RestartLSN, -1, "\"include-lsn\" 'on'", "\"pretty-print\" 'off'")
	if err != nil {
		return err
	}

	// start sending periodic status heartbeats to postgres
	go session.sendPeriodicHeartbeats(ctx)

	for {
		if !session.ReplConn.IsAlive() {
			log.Info("Connection is dead retrying", session.ReplConn.CauseOfDeath())
			if err := session.ReCreateReplConn(); err != nil {
				return err
			}
		}
		log.Info("Waiting for LR message")
		message, err := session.ReplConn.WaitForReplicationMessage(ctx)
		if err != nil {
			// check whether the error is because of the context being cancelled
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return err
		}

		if message.WalMessage != nil {
			walData := message.WalMessage.WalData
			var parsed types.Wal2JSONEvent
			if err := json.Unmarshal(walData, &parsed); err != nil {
				return err
			}
			log.Info("Got message", parsed)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case session.Out <- parsed:
			}

		}

		if message.ServerHeartbeat != nil {
			log.Info("Received server heartbeat", message.ServerHeartbeat)
			// set the flushed LSN (and other LSN values) in the standby status and send to PG
			// send Standby Status if the server is requesting for a reply
			if message.ServerHeartbeat.ReplyRequested == 1 {
				log.Info("Status requested")
				if err = session.sendStandbyStatus(); err != nil {
					log.Error("Unable to send standby status", err)
				}
			}
		}
	}
}

// sendStandbyStatus sends a StandbyStatus object with the current RestartLSN value to the server
func (session *Session) sendStandbyStatus() error {
	standbyStatus, err := pgx.NewStandbyStatus(session.RestartLSN)
	if err != nil {
		return fmt.Errorf("unable to create StandbyStatus object: %s", err)
	}
	log.Info(standbyStatus)
	standbyStatus.ReplyRequested = 0
	log.Info("Sending Standby Status with LSN ", pgx.FormatLSN(session.RestartLSN))
	err = session.ReplConn.SendStandbyStatus(standbyStatus)
	if err != nil {
		return fmt.Errorf("unable to send StandbyStatus object: %s", err)
	}

	return nil
}

// send periodic keep alive hearbeats to the server so that the connection isn't dropped
func (session *Session) sendPeriodicHeartbeats(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(statusHeartbeatIntervalSeconds) * time.Second)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// send hearbeat message at every statusHeartbeatIntervalSeconds interval
			log.Info("Sending periodic status heartbeat")
			err := session.sendStandbyStatus()
			if err != nil {
				log.WithError(err).Error("Failed to send status heartbeat")
			}
		}
	}

}

// LRAckLSN will set the flushed LSN value and trigger a StandbyStatus update
func (session *Session) AckLSN(restartLSNStr string) error {
	restartLSN, err := pgx.ParseLSN(restartLSNStr)
	if err != nil {
		return err
	}
	session.RestartLSN = restartLSN
	return session.sendStandbyStatus()
}
