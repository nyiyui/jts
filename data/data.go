package data

import (
	"slices"
	"time"
)

// Session represents one (possible non-continuous) session of some activity.
// Example: one gaming session, playing Mario Kart
type Session struct {
	ID          string `db:"id"`
	Description string `db:"description"`
	Notes       string `db:"notes"`
	Timeframes  []Timeframe
}

func (s Session) EqualProperties(other Session) bool {
	return s.ID == other.ID && s.Description == other.Description && s.Notes == other.Notes
}

func (s Session) Equal(other Session) bool {
	return s.EqualProperties(other) && slices.EqualFunc(s.Timeframes, other.Timeframes, Timeframe.Equal)
}

type Timeframe struct {
	ID        string    `db:"id"`
	SessionID string    `db:"session_id"`
	Start     time.Time `db:"start_time"`
	End       time.Time `db:"end_time"`
}

func (tf Timeframe) Equal(other Timeframe) bool {
	return tf.ID == other.ID && tf.SessionID == other.SessionID && tf.Start.Equal(other.Start) && tf.End.Equal(other.End)
}

func (tf Timeframe) StringStart() string {
	format := "2006-01-02 15:04"
	if tf.End.Local().YearDay() == tf.Start.Local().YearDay() {
		format = "15:04"
	}
	return tf.Start.Local().Format(format)
}

func (tf Timeframe) StringEnd() string {
	format := "2006-01-02 15:04"
	if tf.End.Local().YearDay() == tf.Start.Local().YearDay() {
		format = "15:04"
	}
	return tf.End.Local().Format(format)
}

func (tf Timeframe) Duration() time.Duration {
	return tf.End.Sub(tf.Start)
}
