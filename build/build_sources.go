package build

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/zengchen1024/obs-worker/sdk/filereceiver"
	"github.com/zengchen1024/obs-worker/sdk/source"
	"github.com/zengchen1024/obs-worker/sdk/sslcert"
	"github.com/zengchen1024/obs-worker/utils"
)

type buildSources struct {
	*buildHelper
}

func (b *buildSources) getSource() (string, error) {
	srcdir := b.getSrcdir()

	if err := mkdir(srcdir); err != nil {
		return "", err
	}

	info := b.getBuildInfo()

	v, err := b.downloadPkgSource(
		info.Project, info.Package, info.SrcMd5,
		b.getSrcServer(), srcdir,
	)
	if err != nil {
		return "", err
	}

	m := make(map[string]string)
	for i := range v {
		item := &v[i]
		m[item.Name] = item.MD5
	}

	if err := b.verify(srcdir, m, info.VerifyMd5); err != nil {
		return "", err
	}

	return genMetaLine(info.VerifyMd5, info.Package), nil
}

func (b *buildSources) verify(dir string, knowns map[string]string, verifyMd5 string) error {
	fs := lsFiles(dir)
	if len(fs) == 0 {
		return fmt.Errorf("didn't download any files")
	}

	sort.Strings(fs)
	s := make([]string, len(fs))

	for i, name := range fs {
		md5, ok := knowns[name]
		if !ok {
			return fmt.Errorf("unexpected file: %s", name)
		}

		s[i] = genMetaLine(md5, name)
	}

	md5 := utils.GenMD5([]byte(strings.Join(s, "\n") + "\n"))

	if md5 != verifyMd5 {
		return fmt.Errorf(
			"source verification fails, %s != %s",
			md5, verifyMd5,
		)
	}

	return nil
}

func (b *buildSources) getBdeps(dir string) ([]string, error) {
	info := b.getBuildInfo()

	if m := info.getkiwimode(); m != "image" && m != "product" {
		return nil, nil
	}

	endpoint := b.getSrcServer()
	items := info.getSrcBDep()
	meta := make([]string, len(items))

	for i, item := range items {
		saveTo := filepath.Join(dir, "images", item.Project, item.Package)
		if err := mkdirAll(saveTo); err != nil {
			return nil, err
		}

		_, err := b.downloadPkgSource(
			item.Project, item.Package, item.SrcMd5,
			endpoint, saveTo,
		)
		if err != nil {
			return nil, err
		}

		meta[i] = genMetaLine(item.SrcMd5, fmt.Sprintf("%s/%s", item.Project, item.Package))
	}

	return meta, nil
}

func (b *buildSources) downloadPkgSource(project, pkg, srcmd5, endpoint, saveTo string) (
	[]filereceiver.CPIOFileMeta, error,
) {
	opts := source.ListOpts{
		Project: project,
		Package: pkg,
		Srcmd5:  srcmd5,
	}

	check := func(name string, h *filereceiver.CPIOFileHeader) (
		string, string, bool, error,
	) {
		return name, filepath.Join(saveTo, name), true, nil
	}

	return source.List(b.gethc(), endpoint, &opts, check)
}

func (b *buildSources) downloadSSLCert() error {
	v, err := sslcert.List(b.gethc(), b.getSrcServer(), b.info.Project, true)
	if err != nil || len(v) == 0 {
		return err
	}

	return utils.WriteFile(
		filepath.Join(b.getSrcdir(), "_projectcert.crt"),
		v,
	)
}
