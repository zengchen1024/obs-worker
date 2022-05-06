package worker

import (
	"encoding/xml"
)

type Worker struct {
	XMLName xml.Name `xml:"worker"`

	RegisterServer string `xml:"registerserver,attr"`
	Hostarch       string `xml:"hostarch"`
	Ip             string `xml:"ip"`
	Workerid       string `xml:"workerid"`
	Owner          string `xml:"owner"`
	Tellnojob      string `xml:"tellnojob"`
	Job            string `xml:"job"`
	Arch           string `xml:"arch"`
	Jobid          string `xml:"jobid"`
	Reposerver     string `xml:"reposerver"`
	Port           int    `xml:"port"`

	BuildArch []string `xml:"buildarch"`
	HostLabel []string `xml:"hostlabel"`
	Sandbox   string   `xml:"sandbox"`

	Linux    Linux    `xml:"linux"`
	Hardware Hardware `xml:"hardware"`
}

type Linux struct {
	XMLName xml.Name `xml:"linux"`

	Version string `xml:"version"`
	Flavor  string `xml:"flavor"`
}

type Hardware struct {
	XMLName xml.Name `xml:"hardware"`

	CPU        CPU `xml:"cpu"`
	Jobs       int `xml:"jobs"`
	Memory     int `xml:"memory"`
	Swap       int `xml:"swap"`
	Disk       int `xml:"disk"`
	Processors int `xml:"processors"`

	NativeOnly bool `xml:"nativeonly"`
}

type CPU struct {
	XMLName xml.Name `xml:"cpu"`

	Flag []string `xml:"flag"`
}

func (w *Worker) Marshal() ([]byte, error) {
	return xml.Marshal(w)
}
