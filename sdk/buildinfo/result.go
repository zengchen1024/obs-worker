package buildinfo

import (
	"encoding/xml"
)

type BuildInfo struct {
	XMLName xml.Name `xml:"buildinfo"`

	Project     string `xml:"project,attr"`
	Repository  string `xml:"repository,attr"`
	Package     string `xml:"package,attr"`
	SrcServer   string `xml:"srcserver,attr"`
	RepoServer  string `xml:"reposerver,attr"`
	DownloadURL string `xml:"downloadurl,attr"`

	Job            string `xml:"job"`
	Arch           string `xml:"arch"`
	HostArch       string `xml:"hostarch"`
	Error          string `xml:"error"`
	SrcMd5         string `xml:"srcmd5"`
	VerifyMd5      string `xml:"verifymd5"`
	Rev            int    `xml:"rev"`
	DistURL        string `xml:"disturl"`
	Reason         string `xml:"reason"`
	Needed         int    `xml:"needed"`
	RevTime        int    `xml:"revtime"`
	ReadyTime      int    `xml:"readytime"`
	Specfile       string `xml:"specfile"`
	File           string `xml:"file"`
	Versrel        string `xml:"versrel"`
	Bcnt           int    `xml:"bcnt"`
	Release        string `xml:"release"`
	DebugInfo      string `xml:"debuginfo"`
	ConstraintsMd5 string `xml:"constraintsmd5"`
	GenMetaAlgo    int    `xml:"genmetaalgo"`
	NoUnchanged    string `xml:"nounchanged"`

	SubPacks  []string `xml:"subpack"`
	ImageType []string `xml:"imagetype"`
	Modules   []string `xml:"module"`
	BDeps     []BDep   `xml:"bdep"`
	Paths     []Path   `xml:"path"`

	FollowupFile string `xml:"followupfile"`
}

func (b *BuildInfo) Marshal() ([]byte, error) {
	return xml.Marshal(b)
}

type BDep struct {
	XMLName xml.Name `xml:"bdep"`

	Name         string `xml:"name,attr"`
	PreInstall   string `xml:"preinstall,attr"`
	VMInstall    string `xml:"vminstall,attr"`
	CbPreInstall string `xml:"cbpreinstall,attr"`
	CbInstall    string `xml:"cbinstall,attr"`
	RunScripts   string `xml:"runscripts,attr"`
	NotMeta      string `xml:"notmeta,attr"`
	NoInstall    string `xml:"noinstall,attr"`
	InstallOnly  string `xml:"installonly,attr"`
	Epoch        string `xml:"epoch,attr"`
	Version      string `xml:"version,attr"`
	Release      string `xml:"release,attr"`
	Arch         string `xml:"arch,attr"`
	HdrMd5       string `xml:"hdrmd5,attr"`
	Project      string `xml:"project,attr"`
	Repository   string `xml:"repository,attr"`
	RepoArch     string `xml:"repoarch,attr"`
	Binary       string `xml:"binary,attr"`
	Package      string `xml:"package,attr"`
	SrcMd5       string `xml:"srcmd5,attr"`
}

func IsTrue(v string) bool {
	return v == "1"
}

type Path struct {
	XMLName xml.Name `xml:"path"`

	Project    string `xml:"project,attr"`
	Repository string `xml:"repository,attr"`
	Server     string `xml:"server,attr"`
	URL        string `xml:"URL,ATTR"`
}

func (r *BuildInfo) Extract(input []byte) error {
	return xml.Unmarshal(input, r)
}
