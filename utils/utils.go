package utils

func SBool(won *bool) bool {
	if won == nil {
		return false
	}
	return *won
}

func IntMin(a, b int) int {
	if a < b {
		return a
	}
	return b
}
