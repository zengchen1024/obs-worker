package image

type Image struct {
	Prpa    string
	SizeK   string
	HdrMD5  string
	Package string
	HdrMD5s []string
	File    string
	Path    string
}

func extract(input []byte) ([]Image, error) {
	return nil, nil
}
