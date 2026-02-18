package db

import "context"

const isFlowAccountingEnabled = false

func (db *Database) IsFlowAccountingEnabled(ctx context.Context) (bool, error) {
	return isFlowAccountingEnabled, nil
}
