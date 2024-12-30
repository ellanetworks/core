package db

import (
	"os"
)

func (db *Database) GetSize() (int64, error) {
	fileInfo, err := os.Stat(db.filepath)
	if err != nil {
		return 0, err
	}
	return fileInfo.Size(), nil
}
