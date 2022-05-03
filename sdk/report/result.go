package report

import (
	"encoding/xml"
)

type Report struct {
	XMLName xml.Name `xml:"size"`

	Epoch      string   `xml:"epoch"`
	Version    string   `xml:"version"`
	Release    string   `xml:"release"`
	Binaryarch string   `xml:"binaryarch"`
	BuildTime  int      `xml:"buildtime"`
	BuildHost  string   `xml:"buildhost"`
	DistURL    string   `xml:"disturl"`
	Binaryid   string   `xml:"binaryid"`
	Binaries   []Binary `xml:"binary"`
}

func (r *Report) Marshal() ([]byte, error) {
	return xml.Marshal(r)
}

type Binary struct {
	XMLName xml.Name `xml:"binary"`

	Name          string `xml:"name"`
	Epoch         string `xml:"epoch"`
	Version       string `xml:"version"`
	Release       string `xml:"release"`
	BinaryArch    string `xml:"binaryarch"`
	Buildtime     string `xml:"buildtime"`
	Buildhost     string `xml:"buildhost"`
	DistURL       string `xml:"disturl"`
	Binaryid      string `xml:"binaryid"`
	Supportstatus string `xml:"supportstatus"`
	Project       string `xml:"project"`
	Repository    string `xml:"repository"`
	Package       string `xml:"package"`
	Arch          string `xml:"arch"`
}

func Extract(input []byte) (r Report, err error) {
	err = xml.Unmarshal(input, &r)
	return
}
