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

func (h *nonModeBinary) getBinaries(considerPreInstallImg bool) ([]string, error) {
	if dir := h.getPkgdir(); !isFileExist(dir) {
		if err := mkdir(dir); err != nil {
			return nil, err
		}
	}

	v := h.info.getNotSrcBDep()
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
		imageBins, imagesWithMeta = h.filterByPreInstallImage(todo)
	}

	done, metas := h.getBinaryCache(todo)

	if todo.Len() > 0 {
		return nil, fmt.Errorf(
			"missing packages: %s",
			strings.Join(todo.UnsortedList(), ", "),
		)
	}

	imagesWithMeta = imagesWithMeta.Union(metas)

	return h.genMetaData(done, imageBins)
}

func (h *nonModeBinary) filterByPreInstallImage(todo sets.String) (
	imageBins map[string]string,
	imagesWithMeta sets.String,
) {
	img := h.imageManager.getPreInstallImage(todo.UnsortedList())
	if img.isEmpty() {
		return
	}

	if h.handleImage != nil {
		h.handleImage(&img)
	}

	imageBins, imagesWithMeta, _ = img.getImageBins()

	if h.handleOutBDep != nil {
		//TODO
	}

	for k := range todo {
		if _, ok := imageBins[k]; ok {
			todo.Delete(k)
		}
	}

	return
}

func (h *nonModeBinary) getBinaryCache(bins sets.String) (map[string]string, sets.String) {
	done := make(map[string]string)
	imagesWithMeta := sets.NewString()

	info := h.getBuildInfo()
	for i := range info.Paths {
		if bins.Len() == 0 {
			break
		}

		repo := &info.Paths[i]

		got, err := h.binaryManager.get(
			h.getPkgdir(), repo, bins.UnsortedList(),
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

			if h.handleOutBDep != nil {
				//TODO
			}

			if h.handleKiwiOrigin != nil {
				h.handleKiwiOrigin(k, prpa)
			}
		}
	}

	return done, imagesWithMeta
}

func (h *nonModeBinary) genMetaData(done, imageBins map[string]string) ([]string, error) {
	subpacks := h.wrapsSubPacks()

	info := h.info
	v := info.getMetaBDep()
	bdeps := make([]string, len(v))
	for i := range v {
		bdeps[i] = v[i].Name
	}

	sort.Strings(bdeps)

	meta := []string{}

	for _, bdep := range bdeps {
		f := filepath.Join(h.getPkgdir(), bdep+".meta")
		if b, err := isEmptyFile(f); b || err != nil {
			v := imageBins[bdep]
			if v == "" {
				if v = queryHdrmd5(filepath.Join(h.getPkgdir(), done[bdep])); v == "" {
					v = "deaddeaddeaddeaddeaddeaddeaddead"
				}
			}

			meta = append(meta, v)
		} else {
			m := h.parseMetaFile(bdep, f, info.Package, subpacks)
			meta = append(meta, m...)
		}
	}

	return h.genMeta(meta, 0), nil
}

func (d *nonModeBinary) parseMetaFile(dep, file, currentPkg string, subpacks sets.String) []string {
	meta := []string{}
	firstLine := true
	seen := sets.NewString()
	isNotSubpack := !subpacks.Has(fmt.Sprintf("/%s/", dep))

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

	readFileLineByLine(file, handle)

	return meta
}

func (h *nonModeBinary) wrapsSubPacks() sets.String {
	info := h.info

	v := sets.NewString()
	for _, item := range info.SubPacks {
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

func (d *nonModeBinary) genMeta(deps []string, algorithm int) []string {
	subpacks := d.wrapsSubPacks()

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

	return helper.genMeta(algorithm)
}
