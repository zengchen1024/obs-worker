package image

import (
	"fmt"
	"path/filepath"

	"sigs.k8s.io/yaml"

	"github.com/zengchen1024/obs-worker/utils"
)

type Image struct {
	SizeK   int      `json:"sizek"`
	Prpa    string   `json:"prpa"`
	File    string   `json:"file"`
	Path    string   `json:"path"`
	HdrMD5  string   `json:"hdrmd5"`
	Package string   `json:"package"`
	HdrMD5s []string `json:"hdrmd5s"`
}

func extract(input []byte, workDir string) ([]Image, error) {
	var images struct {
		Images []Image `json:"images"`
	}

	v, err, _ := utils.RunCmd(
		"perl", "-I", filepath.Join(workDir, "perl"),
		filepath.Join(workDir, "perl", "parse_image_info.pm"),
		string(input),
	)
	if err != nil {
		return nil, fmt.Errorf("%s, %v", v, err)
	}

	if err = yaml.Unmarshal(v, &images); err != nil {
		return nil, err
	}

	return images.Images, nil
}
