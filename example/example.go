package main

import (
	"context"
	"flag"
	"fmt"

	"github.com/nikunjy/pgdeltastream/db"
	"github.com/nikunjy/pgdeltastream/types"
)

func main() {
	dbName := flag.String("db", "postgres", "Name of the database to connect to")
	pgUser := flag.String("user", "postgres", "Postgres user name")
	pgPass := flag.String("password", "", "Postgres password")
	pgHost := flag.String("pgHost", "localhost", "Postgres server hostname")
	pgPort := flag.Int("pgPort", 5432, "Postgres server port")
	flag.Parse()

	cfg := &db.Config{
		SlotName: "test_slot",
	}
	cfg = cfg.WithDB(*dbName, *pgUser, *pgPass, *pgHost, *pgPort)
	session, err := db.NewSession(cfg, nil)
	if err != nil {
		panic(err)
	}
	session.Out = make(chan types.Wal2JSONEvent)

	if err := session.EnsureSlot(); err != nil {
		panic(err)
	}
	fmt.Println("Session details", session.RestartLSN, session.SnapshotName)
	//ctx := context.Background()

	go func() {
		err := session.LRStream(context.Background())
		panic(err)
	}()
	for {
		data := <-session.Out
		fmt.Println("Got session out data", data)
		if err := session.AckLSN(data.NextLSN); err != nil {
			panic(err)
		}
	}
}
