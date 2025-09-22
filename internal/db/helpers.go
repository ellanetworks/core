package db

type ListArgs struct {
	Limit  int `db:"limit"`
	Offset int `db:"offset"`
}
