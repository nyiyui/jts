package data

import "time"

// Session represents one (possible non-continuous) session of some activity.
// Example: one gaming session, playing Mario Kart
type Session struct {
	Description string `sql:"description"`
	Timeframes  []Timeframe
}

type Timeframe struct {
	Start time.Time `sql:"start_time"`
	End   time.Time `sql:"end_time"`
}

func (tf Timeframe) Duration() time.Duration {
	return tf.End.Sub(tf.Start)
}
