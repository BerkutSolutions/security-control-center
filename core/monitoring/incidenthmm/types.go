package incidenthmm

type State int

const (
	StateNormal State = iota
	StateDegraded
	StateOutage
	stateCount
)

func (s State) String() string {
	switch s {
	case StateNormal:
		return "normal"
	case StateDegraded:
		return "degraded"
	case StateOutage:
		return "outage"
	default:
		return "unknown"
	}
}

type Observation int

const (
	ObsOK Observation = iota
	ObsLatencyHigh
	ObsLatencyVeryHigh
	ObsHTTP5xx
	ObsTimeout
	ObsDNS
	ObsConnect
	ObsConnectionRefused
	ObsOtherDown
	obsCount
)

func (o Observation) String() string {
	switch o {
	case ObsOK:
		return "ok"
	case ObsLatencyHigh:
		return "latency_high"
	case ObsLatencyVeryHigh:
		return "latency_very_high"
	case ObsHTTP5xx:
		return "http_5xx"
	case ObsTimeout:
		return "timeout"
	case ObsDNS:
		return "dns"
	case ObsConnect:
		return "connect"
	case ObsConnectionRefused:
		return "connection_refused"
	case ObsOtherDown:
		return "other_down"
	default:
		return "unknown"
	}
}

type Vec3 [stateCount]float64

