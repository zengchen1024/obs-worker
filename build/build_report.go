package build

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/zengchen1024/obs-worker/sdk/report"
	"github.com/zengchen1024/obs-worker/utils"
)

type buildReport struct {
	*buildHelper

	needCollectOrigins bool
	kiwiOrigins        map[string][]string
}

func (b *buildReport) setKiwiOrigin(k, v string) {
	if !b.needCollectOrigins {
		return
	}

	if b.kiwiOrigins == nil {
		b.kiwiOrigins = make(map[string][]string)
	}

	if items, ok := b.kiwiOrigins[k]; ok {
		b.kiwiOrigins[k] = append(items, v)
	} else {
		b.kiwiOrigins[k] = []string{v}
	}
}

func (b *buildReport) do(dir string) {
	if !b.needCollectOrigins {
		return
	}

	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return filepath.SkipDir
		}

		if !info.Mode().IsRegular() {
			return nil
		}

		r := report.Report{}

		err = readFileLineByLine(path, func(l string) bool {
			if bin, ok := b.parseBinary(l); ok {
				r.Binaries = append(r.Binaries, bin)
			}

			return false
		})
		if err != nil {
			return nil
		}

		b.addReportData(&r)

		if o, err := r.Marshal(); err == nil {
			name := filepath.Base(path)
			name = strings.Replace(name, ".packages", ".report", 1)

			tmp := filepath.Join(filepath.Dir(path), name+".new")
			if nil == utils.WriteFile(tmp, o) {
				os.Rename(tmp, strings.TrimSuffix(tmp, ".new"))
			}
		}

		return nil
	})
}

func (b *buildReport) parseBinary(l string) (report.Binary, bool) {
	v := strings.Split(l, "|")
	if len(v) < 6 || v[0] == "gpg-pubkey" {
		return report.Binary{}, false
	}

	bin := report.Binary{
		Name:       v[0],
		Version:    v[2],
		Release:    v[3],
		BinaryArch: v[4],
	}

	if s := v[5]; s != "(none)" && s != "None" {
		bin.DistURL = s
	}

	if s := v[1]; s != "" && s != "(none)" && s != "None" {
		bin.Epoch = s
	}

	if v[1] == "None" && v[3] == "None" {
		//s := v[2]

	}

	return bin, true
}

func (b *buildReport) addReportData(r *report.Report) {
	info := b.getBuildInfo()

	if info.Release != "" {
		r.Release = info.Release
	}

	r.BuildTime = info.BuildTime
	r.BuildHost = info.BuildHost
	r.DistURL = info.DistURL
}
