package statistic

import (
	"encoding/xml"
)

type BuildStatistics struct {
	XMLName xml.Name `xml:"buildstatistics"`

	Disk     Disk     `xml:"disk"`
	Memory   Memory   `xml:"memory"`
	Times    Times    `xml:"times"`
	Download Download `xml:"download"`
}

type Disk struct {
	XMLName xml.Name `xml:"disk"`

	Usage Usage `xml:"usage"`
}

type Usage struct {
	XMLName xml.Name `xml:"usage"`

	Size       Size `xml:"size"`
	IORequests int  `xml:"io_requests"`
	IOSectors  int  `xml:"io_sectors"`
}

type Memory struct {
	XMLName xml.Name `xml:"memory"`

	Usage Size `xml:"usage>size"`
}

type Times struct {
	XMLName xml.Name `xml:"times"`

	Total      Time `xml:"total>time"`
	Preinstall Time `xml:"preinstall>time"`
	Install    Time `xml:"install>time"`
	Main       Time `xml:"main>time"`
	Postchecks Time `xml:"postchecks>time"`
	Rpmlint    Time `xml:"rpmlint>time"`
	Buildcmp   Time `xml:"buildcmp>time"`
	Deltarpms  Time `xml:"deltarpms>time"`
	Download   Time `xml:"download>time"`
}

type Download struct {
	XMLName xml.Name `xml:"download"`

	Size            Size   `xml:"size"`
	Cachehits       int    `xml:"cachehits"`
	Binaries        int    `xml:"binaries"`
	PreinstallImage string `xml:"preinstallimage"`
}

type Time struct {
	XMLName xml.Name `xml:"time"`

	Unit  string `xml:"unit,attr"`
	Value int    `xml:",chardata"`
}

func (t *Time) IsEmpty() bool {
	return t.Unit == ""
}

type Size struct {
	XMLName xml.Name `xml:"size"`

	Unit  string `xml:"unit,attr"`
	Value int    `xml:",chardata"`
}

func (s *Size) IsEmpty() bool {
	return s.Unit == ""
}

func Extract(input []byte) (r BuildStatistics, err error) {
	err = xml.Unmarshal(input, &r)
	return
}

func Mashal(s *BuildStatistics) ([]byte, error) {
	return xml.Marshal(s)
}
