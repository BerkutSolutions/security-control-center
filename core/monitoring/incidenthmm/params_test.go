package incidenthmm

import "testing"

func TestWithDiagBoost_NormalizesTransitions(t *testing.T) {
	base := DefaultParams()
	p := WithDiagBoost(base, 1.5)
	for i := 0; i < int(stateCount); i++ {
		sum := 0.0
		for j := 0; j < int(stateCount); j++ {
			if p.A[i][j] < 0 {
				t.Fatalf("negative transition: row=%d col=%d v=%v", i, j, p.A[i][j])
			}
			sum += p.A[i][j]
		}
		if sum < 0.999999 || sum > 1.000001 {
			t.Fatalf("row not normalized: row=%d sum=%v A=%v", i, sum, p.A[i])
		}
	}
}

