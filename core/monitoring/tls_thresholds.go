package monitoring

func crossedTLSExpiringThreshold(prevDays, nextDays int, thresholds []int) (int, bool) {
	if len(thresholds) == 0 {
		return 0, false
	}
	crossed := 0
	ok := false
	for _, threshold := range thresholds {
		if threshold <= 0 {
			continue
		}
		if nextDays <= threshold && prevDays > threshold {
			if !ok || threshold < crossed {
				crossed = threshold
				ok = true
			}
		}
	}
	return crossed, ok
}
