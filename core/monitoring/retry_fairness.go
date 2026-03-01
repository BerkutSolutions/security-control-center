package monitoring

func retryStartBudget(maxConcurrent int, normalDue int) int {
	if maxConcurrent <= 0 {
		maxConcurrent = 1
	}
	if normalDue <= 0 {
		return maxConcurrent
	}
	budget := maxConcurrent / 3
	if budget < 1 {
		budget = 1
	}
	return budget
}
