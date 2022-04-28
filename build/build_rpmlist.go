package build

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/zengchen1024/obs-worker/sdk/buildinfo"
)

type buildRpmlist struct {
	*buildHelper

	img preInstallImage
}

func (b *buildRpmlist) setPreInstallImage(img *preInstallImage) {
	b.img = *img
}

func (b *buildRpmlist) generate() error {
	pkgdir := b.getPkgdir()
	kiwiMode := b.info.getkiwimode()
	imageBins, _, _ := b.img.getImageBins()

	rpmList := []string{}

	bdeps := b.info.BDeps
	for i := range bdeps {
		bdep := &bdeps[i]

		if bdep.Package != "" || bdep.RepoArch == "src" {
			continue
		}

		if kiwiMode != "" && buildinfo.IsTrue(bdep.NoInstall) {
			continue
		}

		bin := bdep.Name

		if imageBins[bin] != "" {
			rpmList = append(rpmList, fmt.Sprintf("%s preinstallimage", bin))
			continue
		}

		for j, suf := range knownBins {
			if f := filepath.Join(pkgdir, bin+suf); isFileExist(f) {
				rpmList = append(rpmList, fmt.Sprintf("%s %s", bin, f))
				break
			}

			if j == len(knownBins)-1 {
				return fmt.Errorf("missing package: %s", bin)
			}
		}
	}

	if img := &b.img; !img.isEmpty() {
		if s := img.getImageName(); s != "" {
			rpmList = append(
				rpmList,
				fmt.Sprintf("preinstallimage: %s", filepath.Join(pkgdir, s)),
			)
		}

		if s := img.getImageSource(); s != "" {
			rpmList = append(
				rpmList,
				fmt.Sprintf("preinstallimagesource: %s", s),
			)
		}
	}

	if s := b.cfg.LocalKiwi; s != "" {
		if f := filepath.Join(s, s+".rpm"); isFileExist(f) {
			rpmList = append(rpmList, fmt.Sprintf("%s %s", s, f))
		}
	}

	add := func(item string, ok func(*BDep) bool) {
		names := make([]string, 0, len(bdeps))

		for i := range bdeps {
			if bdep := &bdeps[i]; ok(bdep) {
				names = append(names, bdep.Name)
			}
		}

		if len(names) > 0 {
			rpmList = append(
				rpmList,
				fmt.Sprintf("%s: %s", item, strings.Join(names, " ")),
			)
		}
	}

	add("preinstall", func(v *BDep) bool {
		return buildinfo.IsTrue(v.PreInstall)
	})

	add("vminstall", func(v *BDep) bool {
		return buildinfo.IsTrue(v.VMInstall)
	})

	add("runscripts", func(v *BDep) bool {
		return buildinfo.IsTrue(v.RunScripts)
	})

	if kiwiMode != "" {
		add("noinstall", func(v *BDep) bool {
			return buildinfo.IsTrue(v.NoInstall)
		})

		add("installonly", func(v *BDep) bool {
			return buildinfo.IsTrue(v.InstallOnly)
		})
	}

	writeFile(
		b.env.rpmList,
		[]byte(strings.Join(append(rpmList, "\n"), "\n")),
	)

	return nil
}
