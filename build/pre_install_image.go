package build

import (
	"fmt"
	"strings"

	"github.com/zengchen1024/obs-worker/sdk/image"
	"k8s.io/apimachinery/pkg/util/sets"
)

func getImageFile(img *image.Image) string {
	return "preinstallimage." + img.File
}

type preInstallImage struct {
	hdrmd5s map[string]string

	img image.Image

	imagesWithMeta sets.String
	imageOrigins   map[string]string
}

func (p *preInstallImage) isEmpty() bool {
	return p.img.File == ""
}

func (p *preInstallImage) getImageName() string {
	return getImageFile(&p.img)
}

func (p *preInstallImage) getImageBins() (map[string]string, sets.String, map[string]string) {
	knownHdrmd5s := sets.NewString(p.img.HdrMD5s...)

	metas := sets.NewString()
	bins := make(map[string]string)
	imageToPrpa := make(map[string]string)
	for k, v := range p.hdrmd5s {
		if knownHdrmd5s.Has(v) {
			bins[k] = v

			if p.imagesWithMeta.Has(k) {
				metas.Insert(k)
			}

			if v1, ok := p.imageOrigins[k]; ok {
				imageToPrpa[k] = v1
			}
		}
	}

	return bins, metas, imageToPrpa
}

func (p *preInstallImage) getImageSource() string {
	prpa := p.img.Prpa

	// strip arch
	s := prpa
	if i := strings.LastIndex(prpa, "/"); i > 0 {
		s = prpa[:i]
	}

	if p.img.Package != "" {
		s = fmt.Sprintf("%s/%s", s, p.img.Package)
	}

	return fmt.Sprintf("%s %s", s, p.img.HdrMD5)
}
