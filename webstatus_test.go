package sweet

import (
	"testing"
	"time"
)

func TestWebstatusCollectedTimeFormatted(t *testing.T) {
	cases := map[time.Time]string{
		time.Now():                            "Now",
		time.Now().Add(10 * time.Second):      "Now",
		time.Now().Add(-10 * time.Second):     "10 seconds",
		time.Now().Add(-59 * time.Second):     "59 seconds",
		time.Now().Add(-60 * time.Second):     "1 minute",
		time.Now().Add(-61 * time.Second):     "1 minute",
		time.Now().Add(-119 * time.Second):    "1 minute",
		time.Now().Add(-120 * time.Second):    "2 minutes",
		time.Now().Add(-3599 * time.Second):   "59 minutes",
		time.Now().Add(-3600 * time.Second):   "1 hour",
		time.Now().Add(-60 * time.Minute):     "1 hour",
		time.Now().Add(-61 * time.Minute):     "1 hour",
		time.Now().Add(-119 * time.Minute):    "1 hour",
		time.Now().Add(-120 * time.Minute):    "2 hours",
		time.Now().Add(-23 * time.Hour):       "23 hours",
		time.Now().Add(-24 * time.Hour):       "1 day",
		time.Now().Add(-47 * time.Hour):       "1 day",
		time.Now().Add(-48 * time.Hour):       "2 days",
		time.Now().Add(-49 * time.Hour):       "2 days",
		time.Now().Add(-400 * 24 * time.Hour): "400 days",
	}
	for oldTime, expected := range cases {
		r := new(Report)
		r.CollectedTime = oldTime
		if expected != r.CollectedTimeFormatted() {
			t.Errorf("Bad time: expected %s but got %s", expected, r.CollectedTimeFormatted())
		}
	}
}

func TestWebstatusChangedTimeFormatted(t *testing.T) {
	cases := map[time.Time]string{
		time.Now():                            "Now",
		time.Now().Add(10 * time.Second):      "Now",
		time.Now().Add(-10 * time.Second):     "10 seconds",
		time.Now().Add(-59 * time.Second):     "59 seconds",
		time.Now().Add(-60 * time.Second):     "1 minute",
		time.Now().Add(-61 * time.Second):     "1 minute",
		time.Now().Add(-119 * time.Second):    "1 minute",
		time.Now().Add(-120 * time.Second):    "2 minutes",
		time.Now().Add(-3599 * time.Second):   "59 minutes",
		time.Now().Add(-3600 * time.Second):   "1 hour",
		time.Now().Add(-60 * time.Minute):     "1 hour",
		time.Now().Add(-61 * time.Minute):     "1 hour",
		time.Now().Add(-119 * time.Minute):    "1 hour",
		time.Now().Add(-120 * time.Minute):    "2 hours",
		time.Now().Add(-23 * time.Hour):       "23 hours",
		time.Now().Add(-24 * time.Hour):       "1 day",
		time.Now().Add(-47 * time.Hour):       "1 day",
		time.Now().Add(-48 * time.Hour):       "2 days",
		time.Now().Add(-49 * time.Hour):       "2 days",
		time.Now().Add(-400 * 24 * time.Hour): "400 days",
	}
	for oldTime, expected := range cases {
		r := new(Report)
		r.ChangedTime = oldTime
		if expected != r.ChangedTimeFormatted() {
			t.Errorf("Bad time: expected %s but got %s", expected, r.ChangedTimeFormatted())
		}
	}
}
