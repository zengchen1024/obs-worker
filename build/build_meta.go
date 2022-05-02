package build

import (
	"fmt"
	"sort"
	"strings"

	"k8s.io/apimachinery/pkg/util/sets"
)

type buildMeta struct {
	md5  string
	path string

	parts         int
	meta          string
	metaOfLastPkg string
}

func (m *buildMeta) init() {
	m.parts = strings.Count(m.path, "/")

	m.meta = genMetaLine(m.md5, m.path)

	if i := strings.LastIndex(m.path, "/"); i >= 0 {
		m.metaOfLastPkg = genMetaLine(m.md5, m.path[i+1:])
	} else {
		m.metaOfLastPkg = m.meta
	}
}

func genMeta(deps []buildMeta, subpacks []string, algorithm int) []string {
	h := &metaHelper{deps: deps}

	h.init(subpacks)

	h.sortDeps()

	items := h.genMeta(algorithm)

	v := make([]string, len(items))
	for i := range items {
		v[i] = items[i].meta
	}

	return v
}

type metaHelper struct {
	deps        []buildMeta
	cycle       sets.String
	subpackPath sets.String
}

func (h *metaHelper) init(subpacks []string) {
	for i := range h.deps {
		h.deps[i].init()
	}

	wrap := make([]string, len(subpacks))
	for i, s := range subpacks {
		wrap[i] = fmt.Sprintf("/%s/", s)
	}

	isMatch := func(path string) bool {
		s := fmt.Sprintf("/%s/", path)

		for _, k := range wrap {
			if strings.Contains(s, k) {
				return true
			}
		}

		return false
	}

	subpackPath := sets.NewString()
	cycle := sets.NewString()

	for _, item := range h.deps {
		if s := item.path; isMatch(s) {
			subpackPath.Insert(item.path)

			if i := strings.Index(s, "/"); i > 0 {
				cycle.Insert(s[:i])
			} else {
				cycle.Insert(s)
			}
		}
	}

	h.subpackPath = subpackPath

	if cycle.Len() > 0 {
		cycle = cycle.Difference(sets.NewString(subpacks...))
	}
	h.cycle = cycle
}

func (h *metaHelper) sortDeps() {
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

func (h *metaHelper) genMeta(algorithm int) []buildMeta {
	if h.cycle.Len() == 0 {
		return h.prune(h.deps, false)
	}

	if algorithm == 0 {
		return h.genMetaWithoutAlgo()
	}

	return h.genMetaWithAlgo()
}

func (h *metaHelper) genMetaWithoutAlgo() []buildMeta {
	deps := make([]buildMeta, 0, len(h.deps))

	h.handleCycle(func(matched bool, item *buildMeta) {
		if !matched {
			deps = append(deps, *item)
		}
	})

	return h.prune(deps, true)
}

func (h *metaHelper) genMetaWithAlgo() []buildMeta {
	cycleSeen := make(map[string]int)

	h.handleCycle(func(matched bool, item *buildMeta) {
		if matched {
			k := item.metaOfLastPkg

			if _, ok := cycleSeen[k]; !ok {
				cycleSeen[k] = item.parts
			}
		}
	})

	deps := h.prune(h.deps, true)

	meta := []buildMeta{}
	for i := range deps {
		item := &deps[i]
		k := item.metaOfLastPkg

		if n, ok := cycleSeen[k]; !ok || item.parts < n {
			meta = append(meta, *item)
		}
	}

	return meta
}

func (h *metaHelper) handleCycle(handle func(bool, *buildMeta)) {
	if h.cycle.Len() == 0 {
		return
	}

	items := h.cycle.UnsortedList()
	wrap := make([]string, len(items))
	for i, s := range items {
		wrap[i] = fmt.Sprintf("/%s/", s)
	}

	isMatch := func(path string) bool {
		s := fmt.Sprintf("%s/", path)

		for _, k := range wrap {
			if strings.Contains(s, k) {
				return true
			}
		}

		return false
	}

	for i := range h.deps {
		item := &h.deps[i]

		handle(isMatch(item.path), item)
	}
}

func (h *metaHelper) prune(deps []buildMeta, subPackCheck bool) []buildMeta {
	depSeen := sets.NewString()
	meta := []buildMeta{}

	for i := range deps {
		item := &deps[i]

		if depSeen.Has(item.metaOfLastPkg) {
			continue
		}

		if subPackCheck && h.subpackPath.Has(item.path) {
			continue
		}

		depSeen.Insert(item.metaOfLastPkg)

		meta = append(meta, *item)
	}

	return meta
}
