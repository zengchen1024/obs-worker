package build

type buildReport struct {
	needCollectOrigins bool
	kiwiOrigins        map[string][]string
}

func (b *buildReport) setKiwiOrigin(k, v string) {
	if !b.needCollectOrigins {
		return
	}

	if items, ok := b.kiwiOrigins[k]; ok {
		b.kiwiOrigins[k] = append(items, v)
	} else {
		b.kiwiOrigins[k] = []string{v}
	}
}

func (b *buildReport) do() {}
