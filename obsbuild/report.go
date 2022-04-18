package obsbuild

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/zengchen1024/obs-worker/sdk/report"
)

type reportHelper struct {
	b *buildOnce
}

func (h *reportHelper) create(dir string) {
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return filepath.SkipDir
		}

		if !info.Mode().IsRegular() {
			return nil
		}

		r := report.Report{}

		err = readFileLineByLine(path, func(l string) bool {
			h.parseBinary(l, &r)
			return false
		})
		if err != nil {
			return nil
		}

		h.addReportData(&r)

		if b, err := report.Mashal(&r); err == nil {
			name := filepath.Base(path)
			name = strings.Replace(name, ".packages", ".report", 1)

			tmp := filepath.Join(filepath.Dir(path), name+".new")
			if nil == writeFile(tmp, b) {
				os.Rename(tmp, strings.TrimSuffix(tmp, ".new"))
			}
		}

		return nil
	})
}

func (h *reportHelper) parseBinary(l string, r *report.Report) {
	v := strings.Split(l, "|")
	if len(v) < 6 || v[0] == "gpg-pubkey" {
		return
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

	r.Binaries = append(r.Binaries, bin)
}

func (h *reportHelper) addReportData(r *report.Report) {
	info := h.b.info

	if info.Release != "" {
		r.Release = info.Release
	}

	r.BuildTime = info.BuildTime
	r.BuildHost = info.BuildHost
	r.DistURL = info.DistURL
}
