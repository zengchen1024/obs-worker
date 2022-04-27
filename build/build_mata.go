package build

import (
	"sort"
	"strings"

	"k8s.io/apimachinery/pkg/util/sets"
)

type metaHelper struct {
	deps        []string
	cycle       sets.String
	subpackPath sets.String
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

		meta = append(meta, line)
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
