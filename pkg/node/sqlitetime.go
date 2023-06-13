package node

import (
	"database/sql/driver"
	"fmt"
	"time"
)

type SQLiteTime time.Time

func (t SQLiteTime) Time() time.Time {
	return time.Time(t)
}

// Value implements the driver Valuer interface.
func (t SQLiteTime) Value() (driver.Value, error) {
	return t.Time().Format(time.RFC3339), nil
}

// Scan implements the sql.Scanner interface.
func (t *SQLiteTime) Scan(value interface{}) error {
	if value == nil {
		*t = SQLiteTime(time.Now())
		return nil
	}

	st, ok := value.(string)
	if !ok {
		return fmt.Errorf("invalid type for SQLiteTime")
	}

	tt, err := time.Parse(time.RFC3339, st)
	if err != nil {
		return err
	}

	*t = SQLiteTime(tt)

	return nil
}
