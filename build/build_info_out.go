package build

import (
	"github.com/zengchen1024/obs-worker/sdk/buildinfo"
	"path/filepath"
)

type buildInfoOut buildinfo.BuildInfo

func (b *buildInfoOut) setBdep(dep BDep) {
	b.BDeps = append(b.BDeps, dep)
}

func (b *buildInfoOut) writeBuildEnv(dir string) {
	if o, err := ((*buildinfo.BuildInfo)(b)).Marshal(); err == nil {
		writeFile(filepath.Join(dir, "_buildenv"), o)
	}
}
