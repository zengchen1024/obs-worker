package build

import (
	"sort"
	"strings"

	"k8s.io/apimachinery/pkg/util/sets"
)

func genMeta(deps []buildMeta, subpacks sets.String, algorithm int) []string {
	h := &metaHelper{deps: deps}

	h.init(subpacks)

	h.sortDeps()

	if algorithm == 0 {
		return h.genMetaWithoutAlgo()
	}

	return h.genMetaWithAlgo()
}

type buildMeta struct {
	md5  string
	path string

	parts     int
	meta      string
	metaOfPkg string
}

func (m *buildMeta) init() {
	m.parts = strings.Count(m.path, "/")

	m.meta = genMetaLine(m.md5, m.path)

	if i := strings.LastIndex(m.path, "/"); i >= 0 {
		m.metaOfPkg = genMetaLine(m.md5, m.path[i+1:])
	} else {
		m.metaOfPkg = m.meta
	}
}

type metaHelper struct {
	deps        []buildMeta
	cycle       sets.String
	subpackPath sets.String
}

func (h *metaHelper) init(subpacks sets.String) {
	subpackPath := sets.NewString()
	cycle := sets.NewString()

	for _, item := range h.deps {
		a := wrapsEachPkg(item.path, true)

		if subpacks.HasAny(a...) {
			subpackPath.Insert(item.path)

			if !subpacks.Has(a[0]) {
				cycle.Insert(a[0])
			}
		}
	}

	h.cycle = cycle
	h.subpackPath = subpackPath
}

func (h *metaHelper) sortDeps() {
	for i := range h.deps {
		h.deps[i].init()
	}

	pos2index := make([]int, len(h.deps))
	for i := range pos2index {
		pos2index[i] = i
	}

	// sort the index of h.deps in pos2index to reduce the copy time when sorting
	// because it is fast to swap the int other than buildMeta
	sort.Slice(pos2index, func(i, j int) bool {
		a := &h.deps[pos2index[i]]
		b := &h.deps[pos2index[j]]

		if a.parts != b.parts {
			return a.parts < b.parts
		}

		if v := strings.Compare(a.path, b.path); v != 0 {
			return v < 0
		}

		return strings.Compare(a.md5, b.md5) <= 0
	})

	// exchange the elements of h.deps.
	// each element of h.deps will move only once.

	index2pos := make([]int, len(pos2index))
	for i, j := range pos2index {
		index2pos[j] = i
	}

	for start, j := range index2pos {
		if start == j || j == -1 {
			continue
		}

		for index2pos[j] != start {
			j = index2pos[j]
		}

		v := h.deps[j]

		for i, k := j, pos2index[j]; k != j; {
			h.deps[i] = h.deps[k]
			index2pos[i] = -1

			i = k
			k = pos2index[i]
		}

		h.deps[start] = v
	}
}

func (h *metaHelper) genMetaWithoutAlgo() []string {
	if h.cycle.Len() > 0 {
		newDeps := make([]buildMeta, 0, len(h.deps))

		for i := range h.deps {
			item := &h.deps[i]

			a := wrapsEachPkg(item.path, false)
			if h.cycle.HasAny(a...) {
				newDeps = append(newDeps, *item)
			}
		}

		h.deps = newDeps
	}

	return toMeta(h.prune())
}

func (h *metaHelper) genMetaWithAlgo() []string {
	cycleSeen := make(map[string]int)

	if h.cycle.Len() > 0 {
		for i := range h.deps {
			item := &h.deps[i]

			a := wrapsEachPkg(item.path, false)
			if h.cycle.HasAny(a...) {
				k := item.metaOfPkg

				if _, ok := cycleSeen[k]; !ok {
					cycleSeen[k] = item.parts
				}
			}
		}
	}

	deps := h.prune()

	if len(cycleSeen) == 0 {
		return toMeta(deps)
	}

	meta := []string{}
	for _, line := range deps {
		k := line.metaOfPkg

		if n, ok := cycleSeen[k]; !ok || line.parts < n {
			meta = append(meta, line.meta)
		}
	}

	return meta
}

func (h *metaHelper) prune() []buildMeta {
	depSeen := sets.NewString()
	meta := []buildMeta{}

	for _, item := range h.deps {
		hk := item.metaOfPkg

		if depSeen.Has(hk) {
			continue
		}

		if h.subpackPath.Has(item.path) {
			continue
		}

		depSeen.Insert(hk)

		meta = append(meta, item)
	}

	return meta
}

func toMeta(items []buildMeta) []string {
	v := make([]string, len(items))
	for i := range items {
		v[i] = items[i].meta
	}

	return v
}
