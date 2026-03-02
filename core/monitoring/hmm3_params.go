package monitoring

import "berkut-scc/core/monitoring/incidenthmm"

func HMM3DefaultParamsWithDiagBoost(boost float64) incidenthmm.Params {
	return incidenthmm.WithDiagBoost(incidenthmm.DefaultParams(), boost)
}

