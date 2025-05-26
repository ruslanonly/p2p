package timestamp

import (
	"time"
)

type DateTimeSeconds int64

type DateTimeNanoseconds int64

func NowDateTimeSeconds() DateTimeSeconds {
	return DateTimeSeconds(time.Now().Unix())
}

func NowDateTimeNanoseconds() DateTimeNanoseconds {
	return DateTimeNanoseconds(time.Now().UnixNano())
}

func (ts DateTimeSeconds) ToTime() time.Time {
	return time.Unix(int64(ts), 0)
}

func (tn DateTimeNanoseconds) ToTime() time.Time {
	return time.Unix(0, int64(tn))
}

func FromTimeDateTimeNanoseconds(t time.Time) DateTimeNanoseconds {
	return DateTimeNanoseconds(t.UnixNano())
}

func DateTimeNanosecondsArrToTimeArr(arr []DateTimeNanoseconds) []time.Time {
	out := make([]time.Time, len(arr))
	for _, ts := range arr {
		out = append(out, ts.ToTime())
	}

	return out
}
