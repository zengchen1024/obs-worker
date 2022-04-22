package build

import "github.com/zengchen1024/obs-worker/sdk/buildinfo"

type buildInfoOut buildinfo.BuildInfo

func (b *buildInfoOut) setBdep(dep BDep) {
	b.BDeps = append(b.BDeps, dep)
}

func (b *buildInfoOut) writeBuildEnv() {

}
