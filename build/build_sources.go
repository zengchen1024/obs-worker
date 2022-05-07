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

func (h *buildSources) getSource() (string, error) {
	srcdir := h.getSrcdir()

	if err := mkdir(srcdir); err != nil {
		return "", err
	}

	info := h.info

	v, err := h.downloadPkgSource(
		info.Project, info.Package, info.SrcMd5,
		h.getSrcServer(), srcdir,
	)
	if err != nil {
		return "", err
	}

	// verify sources
	// TODO: the way to check is different to obs

	keys := make([]string, len(v))
	m := make(map[string]string)
	for i := range v {
		item := &v[i]
		m[item.Name] = item.MD5
		keys[i] = item.Name
	}

	sort.Strings(keys)

	md5s := make([]string, 0, len(v))
	for _, k := range keys {
		if f := filepath.Join(srcdir, k); !isFileExist(f) {
			return "", fmt.Errorf("%s is not exist", f)
		}

		md5s = append(md5s, genMetaLine(m[k], k))
	}

	md5 := utils.GenMD5([]byte(strings.Join(md5s, "\n") + "\n"))
	if md5 != info.VerifyMd5 {
		return "", fmt.Errorf(
			"source verification fails, %s != %s",
			md5, info.VerifyMd5,
		)
	}

	return genMetaLine(info.VerifyMd5, info.Package), nil
}

func (h *buildSources) getBdeps(dir string) ([]string, error) {
	if m := h.info.getkiwimode(); m != "image" && m != "product" {
		return nil, nil
	}

	endpoint := h.getSrcServer()
	items := h.info.getSrcBDep()
	meta := make([]string, len(items))

	for i, item := range items {
		saveTo := filepath.Join(dir, "images", item.Project, item.Package)
		if err := mkdirAll(saveTo); err != nil {
			return nil, err
		}

		_, err := h.downloadPkgSource(
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

func (h *buildSources) downloadPkgSource(project, pkg, srcmd5, endpoint, saveTo string) (
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

	return source.List(h.gethc(), endpoint, &opts, check)
}

func (h *buildSources) downloadSSLCert() error {
	v, err := sslcert.List(h.gethc(), h.getSrcServer(), h.info.Project, true)
	if err != nil || len(v) == 0 {
		return err
	}

	return utils.WriteFile(
		filepath.Join(h.getSrcdir(), "_projectcert.crt"),
		v,
	)
}
