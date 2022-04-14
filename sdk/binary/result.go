package binary

import (
	"encoding/xml"
)

type BinaryVersionList struct {
	XMLName  xml.Name `xml:"binaryversionlist"`
	Binaries []Binary `xml:"binary"`
}

type Binary struct {
	XMLName    xml.Name `xml:"binary"`
	Name       string   `xml:"name,attr"`
	SizeK      int      `xml:"sizek,attr"`
	HdrMD5     string   `xml:"hdrmd5,attr"`
	Error      string   `xml:"error,attr"`
	MetaMD5    string   `xml:"metamd5,attr"`
	LeadSigMD5 string   `xml:"leadsigmd5,attr"`
}

func extract(input []byte) (r BinaryVersionList, err error) {
	err = xml.Unmarshal(input, &r)
	return
}
