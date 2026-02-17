package db

import "context"

const isFlowAccountingEnabled = true

func (db *Database) IsFlowAccountingEnabled(ctx context.Context) (bool, error) {
	return isFlowAccountingEnabled, nil
}
