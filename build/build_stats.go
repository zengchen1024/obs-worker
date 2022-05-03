package build

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/zengchen1024/obs-worker/sdk/statistic"
)

type buildStats struct {
	stats statistic.BuildStatistics

	startTime         int
	downloadStartTime int
}

func (s *buildStats) setPreInstallImage(img *preInstallImage) {
	s.stats.Download.PreinstallImage = img.getImageName()
}

func (s *buildStats) setCacheHit(n int) {
	s.stats.Download.Cachehits = n
}

func (s *buildStats) setBinaryDownloadDetail(num, size int) {
	if num <= 0 {
		return
	}

	s.stats.Download.Binaries += num
	s.stats.Download.Size.Unit = "k"
	s.stats.Download.Size.Value += size
}

func (s *buildStats) recordDownloadStartTime() {
	s.downloadStartTime = int(time.Now().Unix())
}

func (s *buildStats) recordDownloadTime() {
	s.stats.Times.Download = statistic.Time{
		Unit:  "s",
		Value: int(time.Now().Unix()) - s.downloadStartTime,
	}
}

func (s *buildStats) recordStartTime() {
	s.startTime = int(time.Now().Unix())
}

func (s *buildStats) do(dir string) {
	s.stats.Times.Total = statistic.Time{
		Unit:  "s",
		Value: int(time.Now().Unix()) - s.startTime,
	}

	if b, err := s.stats.Marshal(); err == nil {
		tmp := filepath.Join(dir, "_statistics.new")
		if nil == writeFile(tmp, b) {
			os.Rename(tmp, strings.TrimSuffix(tmp, ".new"))
		}
	}
}
