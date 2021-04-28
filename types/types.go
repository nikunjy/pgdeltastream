package types

// SnapshotDataJSON is the struct that binds with an incoming request for snapshot data
type SnapshotDataJSON struct {
	// SlotName is the name of the replication slot for which the snapshot data needs to be fetched
	// (not used as of now, will be useful in multi client setup)
	SlotName string `json:"slotName" binding:"omitempty"`

	Table   string   `json:"table" binding:"required"`
	Offset  *uint    `json:"offset" binding:"exists"`
	Limit   *uint    `json:"limit" binding:"exists"`
	OrderBy *OrderBy `json:"order_by" binding:"exists"`
}

type OrderBy struct {
	Column string `json:"column" binding:"exists"`
	Order  string `json:"order" binding:"exists"`
}

type WalChange struct {
	Kind         string        `json:"kind"`
	Schema       string        `json:"schema"`
	Table        string        `json:"table"`
	ColumnNames  []string      `json:"columnnames"`
	ColumnTypes  []string      `json:"columntypes"`
	ColumnValues []interface{} `json:"columnvalues"`
}

func (w *WalChange) Values() map[string]interface{} {
	ret := make(map[string]interface{}, len(w.ColumnNames))
	for i := 0; i < len(w.ColumnNames); i++ {
		ret[w.ColumnNames[i]] = w.ColumnValues[i]
	}
	return ret
}

type Wal2JSONEvent struct {
	NextLSN string      `json:"nextlsn"`
	Change  []WalChange `json:"change"`
}
