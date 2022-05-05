package opstatus

import (
	"encoding/xml"
)

type Status struct {
	XMLName xml.Name `xml:"status"`

	Code    int    `xml:"code,attr"`
	Summary string `xml:"summary"`
	Details string `xml:"details"`
}

func (s *Status) Marshal() (string, error) {
	v, err := xml.Marshal(s)
	return string(v), err
}
