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
	if dir := b.getPkgdir(); !isFileExist(dir) {
		if err := mkdir(dir); err != nil {
			return nil, err
		}
	}

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
	getMeta := func(bdep string) string {
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

	subpacks := b.wrapsSubPacks()

	meta := []string{}
	for _, bdep := range bdeps {
		if imagesWithMeta.Has(bdep) {
			f := filepath.Join(dir, bdep+".meta")

			m, err := b.parseMetaFile(bdep, f, info.Package, subpacks)
			if err == nil {
				meta = append(meta, m...)

				continue
			}
		}

		meta = append(meta, genMetaLine(getMeta(bdep), bdep))
	}

	return b.genMeta(meta, subpacks), nil
}

func (b *nonModeBinary) parseMetaFile(dep, file, currentPkg string, subpacks sets.String) ([]string, error) {
	if v, err := isEmptyFile(file); v || err != nil {
		return nil, err
	}

	isNotSubpack := !subpacks.Has(fmt.Sprintf("/%s/", dep))
	seen := sets.NewString()
	firstLine := true

	meta := []string{}
	handle := func(line string) bool {
		line = strings.TrimRight(line, "\n") // need it?
		md5, pkg := splitMetaLine(line)

		if firstLine {
			if strings.HasSuffix(line, genMetaLine("", currentPkg)) {
				return true
			}

			meta = append(meta, genMetaLine(md5, dep))

			firstLine = false
			return false
		}

		if isNotSubpack {
			if seen.Has(line) {
				return false
			}

			if a := wrapsEachPkg(pkg, false); !subpacks.HasAny(a...) {
				seen.Insert(line)
			}
		}

		meta = append(meta, genMetaLine(md5, fmt.Sprintf("%s/%s", dep, pkg)))

		return false
	}

	err := readFileLineByLine(file, handle)

	return meta, err
}

func (b *nonModeBinary) genMeta(deps []string, subpacks sets.String) []string {
	subpackPath := sets.NewString()
	cycle := sets.NewString()

	for _, line := range deps {
		_, pkg := splitMetaLine(line)

		if a := wrapsEachPkg(pkg, true); subpacks.HasAny(a...) {
			subpackPath.Insert(pkg)

			if !subpacks.Has(a[0]) {
				cycle.Insert(a[0])
			}
		}
	}

	helper := &metaHelper{
		deps, subpackPath, cycle,
	}

	return helper.genMeta(b.getBuildInfo().GenMetaAlgo)
}

func (b *nonModeBinary) wrapsSubPacks() sets.String {
	v := sets.NewString()
	for _, item := range b.getBuildInfo().SubPacks {
		v.Insert(fmt.Sprintf("/%s/", item))
	}

	return v
}

func wrapsEachPkg(pkgPath string, full bool) []string {
	items := strings.Split(pkgPath, "/")

	n := len(items)
	a := make([]string, n)
	for i := 1; i < n; i++ {
		a[i] = fmt.Sprintf("/%s/", items[i])
	}

	if full {
		a[0] = fmt.Sprintf("/%s/", items[0])
	} else {
		a[0] = fmt.Sprintf("%s/", items[0])
	}

	return a
}
