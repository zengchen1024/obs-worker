package build

import (
	"os"
	"path/filepath"

	"github.com/zengchen1024/obs-worker/sdk/filereceiver"
	"github.com/zengchen1024/obs-worker/sdk/oldpkg"
)

type buildOldPackages struct {
	*buildHelper

	handleDownloadDetails func(int, int)
}

func (b *buildOldPackages) download() error {
	dir := b.env.oldpkgdir

	if err := mkdirAll(dir); err != nil {
		return err
	}

	info := b.getBuildInfo()

	opts := oldpkg.ListOpts{
		Project:    info.Project,
		Repository: info.Repository,
		Arch:       info.Arch,
		Package:    info.Package,
	}

	check := func(name string, h *filereceiver.CPIOFileHeader) (
		string, string, bool, error,
	) {
		return name, filepath.Join(dir, name), false, nil
	}

	r, err := oldpkg.List(info.RepoServer, &opts, check)
	if err != nil {
		return err
	}

	if len(r) == 0 {
		return os.Remove(dir)
	}

	if b.handleDownloadDetails != nil {
		var n int64
		for i := range r {
			n += r[i].Size >> 10
		}

		b.handleDownloadDetails(len(r), int(n))
	}

	return nil
}
