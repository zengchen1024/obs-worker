package workerstate

import (
	"encoding/xml"
)

type WorkerState struct {
	XMLName xml.Name `xml:"workerstate"`

	State        string `xml:"state"`
	Nextstate    string `xml:"nextstate"`
	JobId        string `xml:"jobid"`
	Pid          string `xml:"pid"`
	Logsizelimit string `xml:"logsizelimit"`
	Logidlelimit string `xml:"logidlelimit"`
}

func (s *WorkerState) Marshal() ([]byte, error) {
	return xml.Marshal(s)
}
