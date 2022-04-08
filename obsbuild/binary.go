package obsbuild

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"k8s.io/apimachinery/pkg/util/sets"
)

func (b *buildOnce) getBinaries() error {
	if err := os.Mkdir(b.env.pkgdir, os.FileMode(0777)); err != nil {
		return err
	}

	info := b.info

	v := info.getNotSrcBDep()
	if len(v) == 0 {
		return fmt.Errorf("no binaries needed for this package")
	}

	bdep := make([]string, len(v))
	for i := range v {
		bdep[i] = v[i].Name
	}

	return nil
}

type downloadBinary struct {
	b *buildOnce
}

func (d *downloadBinary) genMetaForNonKiwi() ([]string, error) {
	subpacks := d.wrapsSubPacks()

	info := d.b.info
	v := info.getMetaBDep()
	bdeps := make([]string, len(v))
	for i := range v {
		bdeps[i] = v[i].Name
	}

	sort.Strings(bdeps)

	meta := []string{}

	for _, bdep := range bdeps {
		f := filepath.Join(d.b.env.pkgdir, bdep+".meta")
		if b, err := isEmptyFile(f); b || err != nil {

		} else {
			m := d.parseMetaFile(bdep, f, info.Package, subpacks)
			meta = append(meta, m...)
		}
	}

	return d.genMeta(meta, 0), nil
}

func (d *downloadBinary) parseMetaFile(dep, file, currentPkg string, subpacks sets.String) []string {
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

func (d *downloadBinary) wrapsSubPacks() sets.String {
	info := d.b.info

	v := sets.NewString()
	for _, item := range info.SubPack {
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

func (d *downloadBinary) genMeta(deps []string, algorithm int) []string {
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

type metaHelper struct {
	deps        []string
	subpackPath sets.String
	cycle       sets.String
}

func (h *metaHelper) genMeta(algorithm int) []string {
	sort.Slice(h.deps, func(i, j int) bool {
		a := h.deps[i]
		b := h.deps[j]

		_, patha := splitMetaLine(a)
		_, pathb := splitMetaLine(b)

		n1 := h.pkgPathPartsNum(patha)
		n2 := h.pkgPathPartsNum(pathb)
		if n1 != n2 {
			return n1 < n2
		}

		if v := strings.Compare(patha, pathb); v != 0 {
			return v < 0
		}

		return strings.Compare(a, b) <= 0
	})

	if algorithm == 0 {
		return h.genMetaWithoutAlgo()
	}

	return h.genMetaWithAlgo()
}

func (h *metaHelper) genMetaWithoutAlgo() []string {
	if h.cycle.Len() > 0 {
		newDeps := make([]string, 0, len(h.deps))

		h.handleCycle(func(b bool, line string) {
			if !b {
				newDeps = append(newDeps, line)
			}
		})

		h.deps = newDeps
	}

	return h.prune()
}

func (h *metaHelper) genMetaWithAlgo() []string {
	cycleSeen := make(map[string]int)

	h.handleCycle(func(b bool, line string) {
		if b {
			k := h.genMetaOfLastPkg(line)

			if _, ok := cycleSeen[k]; !ok {
				cycleSeen[k] = h.pkgPathPartsNum(line)
			}
		}
	})

	deps := h.prune()

	if len(cycleSeen) == 0 {
		return deps
	}

	meta := []string{}
	for _, line := range deps {
		k := h.genMetaOfLastPkg(line)

		if n, ok := cycleSeen[k]; !ok || h.pkgPathPartsNum(line) < n {
			meta = append(meta, line)
		}
	}

	return meta
}

func (h *metaHelper) pkgPathPartsNum(line string) int {
	return strings.Count(line, "/")
}

func (h *metaHelper) handleCycle(handle func(bool, string)) {
	if h.cycle.Len() == 0 {
		return
	}

	f := func(line string) []string {
		_, pkg := splitMetaLine(line)
		return wrapsEachPkg(pkg, false)
	}

	for _, line := range h.deps {
		a := f(line)
		handle(h.cycle.HasAny(a...), line)
	}
}

func (h *metaHelper) prune() []string {
	depSeen := sets.NewString()
	meta := []string{}

	for _, line := range h.deps {
		hk := h.genMetaOfLastPkg(line)
		if depSeen.Has(hk) {
			continue
		}

		if _, k := splitMetaLine(line); h.subpackPath.Has(k) {
			continue
		}

		depSeen.Insert(hk)
	}

	return meta
}

func (h *metaHelper) genMetaOfLastPkg(line string) string {
	i := strings.LastIndex(line, "/")
	if i < 0 {
		return line
	}

	md5, _ := splitMetaLine(line)
	return genMetaLine(md5, line[i+1:])
}
