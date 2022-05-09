package workerstate

import (
	"encoding/xml"
)

const (
	WorkerStateIdle      = "idle"
	WorkerStateExit      = "exit"
	WorkerStateKilled    = "killed"
	WorkerStateBadHost   = "badhost"
	WorkerStateBuilding  = "building"
	WorkerStateDiscarded = "discarded"
)

type WorkerState struct {
	XMLName xml.Name `xml:"workerstate"`

	State        string `xml:"state"`
	NextState    string `xml:"nextstate"`
	JobId        string `xml:"jobid"`
	Pid          string `xml:"pid"`
	LogSizeLimit string `xml:"logsizelimit"`
	LogIdleLimit string `xml:"logidlelimit"`
}

func (s *WorkerState) Marshal() ([]byte, error) {
	return xml.Marshal(s)
}
