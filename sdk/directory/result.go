package directory

import (
	"encoding/xml"
)

type Directory struct {
	XMLName xml.Name `xml:"directory"`

	Entries []Entry `xml:"entry"`
}

type Entry struct {
	XMLName xml.Name `xml:"entry"`

	Name  string `xml:"name"`
	Size  int64  `xml:"size"`
	Mtime int64  `xml:"mtime"`
}

func (d *Directory) Marshal() ([]byte, error) {
	return xml.Marshal(d)
}
