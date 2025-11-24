package db

type ListArgs struct {
	Limit  int `db:"limit"`
	Offset int `db:"offset"`
}

type NumItems struct {
	Count int `db:"count"`
}

type cutoffArgs struct {
	Cutoff string `db:"cutoff"`
}

type cutoffDaysArgs struct {
	CutoffDays int64 `db:"cutoff_days"`
}
