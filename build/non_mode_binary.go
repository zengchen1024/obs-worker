package build

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/zengchen1024/obs-worker/utils"
)

type nonModeBinary struct {
	*buildHelper

	binaryManager *binaryManager
	imageManager  *preInstallImageManager

	handleOutBDep    func(BDep)
	handleImage      func(*preInstallImage)
	handleKiwiOrigin func(k, v string)
}

func (b *nonModeBinary) getBinaries(considerPreInstallImg bool) ([]string, error) {
	v := b.getBuildInfo().getNotSrcBDep()
	if len(v) == 0 {
		return nil, fmt.Errorf("no binaries needed for this package")
	}

	todo := sets.NewString()
	for i := range v {
		todo.Insert(v[i].Name)
	}

	var imagesWithMeta sets.String
	var imageBins map[string]string

	if considerPreInstallImg {
		imageBins, imagesWithMeta = b.filterByPreInstallImage(todo)
	}

	done, metas := b.getBinaryCache(todo)

	if todo.Len() > 0 {
		return nil, fmt.Errorf(
			"missing packages: %s",
			strings.Join(todo.UnsortedList(), ", "),
		)
	}

	imagesWithMeta = imagesWithMeta.Union(metas)

	return b.genMetaData(done, imageBins, imagesWithMeta)
}

func (b *nonModeBinary) filterByPreInstallImage(todo sets.String) (
	imageBins map[string]string,
	imagesWithMeta sets.String,
) {
	img := b.imageManager.getPreInstallImage(todo.UnsortedList())
	if img.isEmpty() {
		return
	}

	if b.handleImage != nil {
		b.handleImage(&img)
	}

	imageBins, imagesWithMeta, _ = img.getImageBins()

	if b.handleOutBDep != nil {
		//TODO
	}

	for k := range todo {
		if _, ok := imageBins[k]; ok {
			todo.Delete(k)
		}
	}

	return
}

func (b *nonModeBinary) getBinaryCache(bins sets.String) (map[string]string, sets.String) {
	done := make(map[string]string)
	imagesWithMeta := sets.NewString()

	info := b.getBuildInfo()
	for i := range info.Paths {
		if bins.Len() == 0 {
			break
		}

		repo := &info.Paths[i]

		got, err := b.binaryManager.get(
			b.getPkgdir(), repo, bins.UnsortedList(),
		)
		if err != nil {
			utils.LogErr("get binary with cache, err:%v\n", err)
			continue
		}

		nometa := info.isRepoNoMeta(repo)
		prpa := info.getPrpaOfRepo(repo)

		for k, v := range got {
			done[k] = v.name
			bins.Delete(k)

			if !nometa && v.hasMeta {
				imagesWithMeta.Insert(k)
			}

			if b.handleOutBDep != nil {
				//TODO
			}

			if b.handleKiwiOrigin != nil {
				b.handleKiwiOrigin(k, prpa)
			}
		}
	}

	return done, imagesWithMeta
}

func (b *nonModeBinary) genMetaData(
	done, imageBins map[string]string,
	imagesWithMeta sets.String,
) ([]string, error) {
	info := b.getBuildInfo()

	v := info.getMetaBDep()
	bdeps := make([]string, len(v))
	for i := range v {
		bdeps[i] = v[i].Name
	}

	sort.Strings(bdeps)

	dir := b.getPkgdir()
	getMd5 := func(bdep string) string {
		if v := imageBins[bdep]; v != "" {
			return v
		}

		if s, ok := done[bdep]; ok {
			if v := queryHdrmd5(filepath.Join(dir, s)); v != "" {
				return v
			}
		}

		return "deaddeaddeaddeaddeaddeaddeaddead"
	}

	subpacks := make([]string, len(info.SubPacks))
	for i, s := range info.SubPacks {
		subpacks[i] = fmt.Sprintf("/%s/", s)
	}

	isSubpack := func(p string) bool {
		for _, k := range subpacks {
			if strings.Contains(p, k) {
				return true
			}
		}

		return false
	}

	meta := []buildMeta{}
	for _, bdep := range bdeps {
		if imagesWithMeta.Has(bdep) {
			f := filepath.Join(dir, bdep+".meta")

			m, err := b.parseMetaFile(bdep, f, info.Package, isSubpack)
			if err == nil {
				meta = append(meta, m...)

				continue
			}
		}

		meta = append(meta, buildMeta{
			md5:  getMd5(bdep),
			path: bdep,
		})
	}

	return genMeta(meta, info.SubPacks, info.GenMetaAlgo), nil
}

func (b *nonModeBinary) parseMetaFile(dep, file, currentPkg string, isSubpack func(string) bool) ([]buildMeta, error) {
	if v, err := isEmptyFile(file); v || err != nil {
		return nil, err
	}

	isNotSubpack := !isSubpack(fmt.Sprintf("/%s/", dep))
	seen := sets.NewString()
	firstLine := true

	meta := []buildMeta{}
	add := func(md5, path string) {
		meta = append(meta, buildMeta{
			md5:  md5,
			path: path,
		})
	}

	handle := func(line string) bool {
		line = strings.TrimRight(line, "\n") // need it?
		md5, pkg := splitMetaLine(line)

		if firstLine {
			if strings.HasSuffix(line, genMetaLine("", currentPkg)) {
				return true
			}

			add(md5, dep)

			firstLine = false
			return false
		}

		if isNotSubpack {
			if seen.Has(pkg) {
				return false
			}

			if !isSubpack(pkg + "/") {
				seen.Insert(pkg)
			}
		}

		add(md5, fmt.Sprintf("%s/%s", dep, pkg))

		return false
	}

	err := readFileLineByLine(file, handle)

	return meta, err
}
