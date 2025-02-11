package data

import "time"

// Session represents one (possible non-continuous) session of some activity.
// Example: one gaming session, playing Mario Kart
type Session struct {
	ID          int    `db:"id"`
	Description string `db:"description"`
	Timeframes  []Timeframe
}

type Timeframe struct {
	ID        int       `db:"id"`
	SessionID int       `db:"session_id"`
	Start     time.Time `db:"start_time"`
	End       time.Time `db:"end_time"`
}

func (tf Timeframe) Duration() time.Duration {
	return tf.End.Sub(tf.Start)
}
