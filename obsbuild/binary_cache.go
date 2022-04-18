package obsbuild

import (
	"os"
	"path/filepath"

	"github.com/zengchen1024/obs-worker/sdk/binary"
	"github.com/zengchen1024/obs-worker/utils"
	"k8s.io/apimachinery/pkg/util/sets"
)

type binaryInfo struct {
	name    string
	hdrmd5  string
	hasMeta bool
}

type binaryCacheHelper struct {
	b    *buildOnce
	info *binary.ListOpts

	dir        string
	repoServer string

	knowns []binary.Binary
}

func (h *binaryCacheHelper) get() (map[string]binaryInfo, error) {
	bv := h.getBinaries()
	ret, oldCache, toDownload := h.getDownloads(bv)

	total := 0
	bins := []string{}
	for k, v := range toDownload {
		bins = append(bins, k)
		total += v
	}

	cacheDir := h.b.getCacheDir()
	if cacheDir != "" && len(toDownload) > 0 {
		cacheSize := h.b.getCacheSize()

		if n := total << 10; n*100 > cacheSize {
			h.b.manageCache(cacheSize-n, nil, nil)
		}
	}

	newCaches, err := h.download(bins, ret)
	if err != nil {
		return nil, err
	}

	if cacheDir != "" {
		h.b.manageCache(h.b.getCacheSize(), oldCache, newCaches)
	}

	if h.info.NoMeta {
		for k, v := range ret {
			if v.hasMeta {
				os.Remove(filepath.Join(h.dir, k+".meta"))

				v.hasMeta = false
				ret[k] = v
			}
		}
	}

	return ret, nil
}

func (h *binaryCacheHelper) getBinaries() map[string]*binary.Binary {
	f := func(bl []binary.Binary) map[string]*binary.Binary {
		bv := make(map[string]*binary.Binary)
		for i := range bl {
			item := &bl[i]

			if item.Error != "" {
				bv[item.Name] = item
			} else if bin, _, ok := isBinFile(item.Name); ok {
				bv[bin] = item
			}
		}

		return bv
	}

	bv := f(h.knowns)

	hasAll := true
	for _, k := range h.info.Binaries {
		if _, ok := bv[k]; !ok {
			hasAll = false
			break
		}
	}

	cacheDir := h.b.getCacheDir()

	if cacheDir != "" && !hasAll {
		if v, err := binary.List(&h.b.hc, h.repoServer, h.info); err == nil {
			bv = f(v.Binaries)
		}
	}

	return bv
}

func (h *binaryCacheHelper) getDownloads(bv map[string]*binary.Binary) (
	binaries map[string]binaryInfo,
	oldCache []binCacheInfo,
	toDownload map[string]int,
) {
	info := h.info
	prpa := genPrpa(info.Project, info.Repository, info.Arch)
	cacheDir := h.b.getCacheDir()

	toDownload = make(map[string]int)
	oldCache = []binCacheInfo{}

	for _, binName := range info.Binaries {
		bin, ok := bv[binName]
		if !ok {
			toDownload[binName] = 0
			continue
		}
		if bin.Error != "" {
			continue
		}

		useCache, haveMeta, cache := h.checkInCache(binName, bin, cacheDir, prpa)

		if !useCache {
			toDownload[binName] = bin.SizeK
		} else {
			oldCache = append(oldCache, cache)

			binaries[binName] = binaryInfo{
				name:    bin.Name,
				hdrmd5:  bin.HdrMD5,
				hasMeta: haveMeta,
			}
		}
	}

	return
}

func (h *binaryCacheHelper) checkInCache(binName string, bin *binary.Binary, cacheDir, prpa string) (
	useCache bool,
	haveMeta bool,
	cache binCacheInfo,
) {
	if cacheDir == "" {
		return
	}

	cacheId := genCacheId(prpa, bin.HdrMD5)
	cacheFile := genCacheFile(cacheDir, cacheId)

	to := filepath.Join(h.dir, bin.Name)
	if nil != linkOrCopy(cacheFile, to) {
		return
	}

	useCache = true

	if !h.info.NoMeta && bin.HdrMD5 != "" {
		tmp := filepath.Join(h.dir, binName+".meta")

		if nil == linkOrCopy(cacheFile+".meta", tmp) {
			md5, _ := utils.GenMd5OfFile(tmp)

			if md5 == bin.MetaMD5 {
				haveMeta = true
			} else {
				os.Remove(tmp)
			}
		}

		if !haveMeta {
			useCache = false
		}
	}

	if useCache && queryHdrmd5(to) == bin.HdrMD5 {
		stat, _ := os.Stat(to)
		cache = binCacheInfo{cacheId, int(stat.Size())}

		return
	}

	useCache = false
	os.Remove(to)

	return
}

func (h *binaryCacheHelper) download(
	toDownload []string,
	binaries map[string]binaryInfo,
) ([]binCache, error) {
	if len(toDownload) == 0 {
		return nil, nil
	}

	opts := binary.DownloadOpts{
		CommonOpts: h.info.CommonOpts,
	}
	opts.Binaries = toDownload

	res, err := binary.Download(&h.b.hc, h.repoServer, &opts, h.dir)
	if err != nil {
		// TODO: ignore 404
		// log it
		return nil, err
	}

	info := h.info
	prpa := genPrpa(info.Project, info.Repository, info.Arch)

	haveMeta := sets.NewString()
	newCaches := []binCache{}

	for i := range res {
		name := res[i].Name

		if bin, _, ok := isBinFile(name); ok {
			tmp := filepath.Join(h.dir, name)
			stat, _ := os.Stat(tmp)
			md5, _ := utils.GenMd5OfFile(tmp)

			newCaches = append(newCaches, binCache{
				binCacheInfo: binCacheInfo{
					genCacheId(prpa, md5),
					int(stat.Size()),
				},
				binFile: tmp,
			})

			binaries[bin] = binaryInfo{
				name:   name,
				hdrmd5: md5,
			}

		} else if bin, ok := isMetaFile(name); ok {
			haveMeta.Insert(bin)
		}
	}

	for k := range haveMeta {
		if v, ok := binaries[k]; ok {
			v.hasMeta = true
			binaries[k] = v
		}
	}

	return newCaches, nil
}
