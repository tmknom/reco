package apply

import "time"

// delayRecorder は、待つよう求められた時間を記録し、実時間を待たずに経過を知らせる。
type delayRecorder struct {
	delays []time.Duration
}

func (r *delayRecorder) after(d time.Duration) <-chan time.Time {
	r.delays = append(r.delays, d)
	elapsed := make(chan time.Time, 1)
	elapsed <- time.Time{}
	return elapsed
}

// elapsedAtOnce は、待つ時間を確かめないテストで、実時間を待たずに経過を知らせる。
func elapsedAtOnce(d time.Duration) <-chan time.Time {
	return (&delayRecorder{}).after(d)
}
