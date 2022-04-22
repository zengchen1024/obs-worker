package build

import (
	"fmt"
	"strings"

	"github.com/zengchen1024/obs-worker/sdk/config"
	"github.com/zengchen1024/obs-worker/utils"
)

type buildHelper struct {
	cfg  *Config
	hc   utils.HttpClient
	info BuildInfo
	env  buildEnv
}

func (h *buildHelper) gethc() *utils.HttpClient {
	return &h.hc
}

func (h *buildHelper) getBuildInfo() *BuildInfo {
	return &h.info
}

func (h *buildHelper) getSrcServer() string {
	if h.info.SrcServer != "" {
		return h.info.SrcServer
	}

	return h.cfg.SrcServer
}

func (h *buildHelper) getWorkerId() string {
	return h.cfg.Id
}

func (h *buildHelper) getCacheDir() string {
	return h.cfg.CacheDir
}

func (h *buildHelper) getCacheSize() int {
	return h.cfg.CacheSize
}

func (h *buildHelper) getPkgdir() string {
	return h.env.pkgdir
}

func (h *buildHelper) getSrcdir() string {
	return h.env.srcdir
}

func (b *buildHelper) downloadProjectConfig() error {
	return config.Download(
		&b.hc,
		b.getSrcServer(),
		&config.DownloadOpts{
			Project:    b.info.Project,
			Repository: b.info.Repository,
		},
		b.env.config,
	)
}

func genPrpa(proj, repo, arch string) string {
	return fmt.Sprintf("%s/%s/%s", proj, repo, arch)
}

func pasePrpa(s string) (string, string, string) {
	if v := strings.Split(s, "/"); len(v) == 3 {
		return v[0], v[1], v[2]
	}

	return s, "", ""
}
