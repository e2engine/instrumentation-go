package e2engine

const testExecutionIDLength = 64

// ValidTestExecutionID reports whether id is a valid E2Engine test execution ID.
//
// A valid ID contains exactly 64 lowercase alphanumeric ASCII characters.
func ValidTestExecutionID(id string) bool {
	if len(id) != testExecutionIDLength {
		return false
	}

	for i := 0; i < len(id); i++ {
		c := id[i]

		if (c < '0' || c > '9') &&
			(c < 'a' || c > 'z') {
			return false
		}
	}

	return true
}
