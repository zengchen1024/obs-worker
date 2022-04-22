package build

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

type binaryManager struct {
	*buildHelper

	knowns map[string]binary.BinaryVersionList

	handleCacheHits       func(int)
	handleDownloadDetails func(num, size int)
}

func (h *binaryManager) init() {
	h.knowns = make(map[string]binary.BinaryVersionList)
}

func (h *binaryManager) getHttpClient() *utils.HttpClient {
	return &h.hc
}

func (h *binaryManager) setKnownBins(prpa string, bins binary.BinaryVersionList) {
	h.knowns[prpa] = bins
}

func (h *binaryManager) get(
	dir, repoServer string,
	binInfo *binary.ListOpts,
) (map[string]binaryInfo, error) {
	bv := h.getBinaries(repoServer, binInfo)

	ret, oldCache, toDownload := h.getDownloads(dir, binInfo, bv)

	total := 0
	bins := []string{}
	for k, v := range toDownload {
		bins = append(bins, k)
		total += v
	}

	manager := cacheManager{h.buildHelper}

	cacheDir := h.getCacheDir()
	if cacheDir != "" && len(toDownload) > 0 {
		cacheSize := h.getCacheSize()

		if n := total << 10; n*100 > cacheSize {
			manager.pruneCache(cacheSize-n, nil, nil)
		}
	}

	newCaches, err := h.download(dir, repoServer, binInfo, bins, ret)
	if err != nil {
		return nil, err
	}

	if cacheDir != "" {
		manager.pruneCache(h.getCacheSize(), oldCache, newCaches)
	}

	if binInfo.NoMeta {
		for k, v := range ret {
			if v.hasMeta {
				os.Remove(filepath.Join(dir, k+".meta"))

				v.hasMeta = false
				ret[k] = v
			}
		}
	}

	return ret, nil
}

func (h *binaryManager) getBinaries(
	repoServer string,
	binInfo *binary.ListOpts,
) map[string]*binary.Binary {
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

	prpa := genPrpa(binInfo.Project, binInfo.Repository, binInfo.Arch)
	if v, ok := h.knowns[prpa]; ok {
		hasAll := true
		bv := f(v.Binaries)

		for _, k := range binInfo.Binaries {
			if _, ok := bv[k]; !ok {
				hasAll = false
				break
			}
		}

		if hasAll {
			return bv
		}
	}

	if cacheDir := h.getCacheDir(); cacheDir != "" {
		v, err := binary.List(h.getHttpClient(), repoServer, binInfo)
		if err == nil {
			h.knowns[prpa] = v

			return f(v.Binaries)
		}
	}

	return nil
}

func (h *binaryManager) getDownloads(dir string, binInfo *binary.ListOpts, bv map[string]*binary.Binary) (
	binaries map[string]binaryInfo,
	oldCache []cacheBinInfo,
	toDownload map[string]int,
) {
	prpa := genPrpa(binInfo.Project, binInfo.Repository, binInfo.Arch)
	cacheDir := h.getCacheDir()

	toDownload = make(map[string]int)
	oldCache = []cacheBinInfo{}

	for _, binName := range binInfo.Binaries {
		bin, ok := bv[binName]
		if !ok {
			toDownload[binName] = 0
			continue
		}
		if bin.Error != "" {
			continue
		}

		useCache, haveMeta, cache := h.checkInCache(
			dir, binName, bin, cacheDir, prpa, binInfo.NoMeta,
		)

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

func (h *binaryManager) checkInCache(
	dir, binName string, bin *binary.Binary, cacheDir, prpa string, nometa bool,
) (
	useCache bool,
	haveMeta bool,
	cache cacheBinInfo,
) {
	if cacheDir == "" {
		return
	}

	cacheId := genCacheId(prpa, bin.HdrMD5)
	cacheFile := genCacheFile(cacheDir, cacheId)

	to := filepath.Join(dir, bin.Name)
	if nil != linkOrCopy(cacheFile, to) {
		return
	}

	useCache = true

	if !nometa && bin.HdrMD5 != "" {
		tmp := filepath.Join(dir, binName+".meta")

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
		cache = cacheBinInfo{cacheId, int(stat.Size())}

		return
	}

	useCache = false
	os.Remove(to)

	return
}

func (h *binaryManager) download(
	dir, repoServer string,
	binInfo *binary.ListOpts,
	toDownload []string,
	binaries map[string]binaryInfo,
) ([]cacheBin, error) {
	if len(toDownload) == 0 {
		return nil, nil
	}

	opts := binary.DownloadOpts{
		CommonOpts: binInfo.CommonOpts,
	}
	opts.Binaries = toDownload

	res, err := binary.Download(h.getHttpClient(), repoServer, &opts, dir)
	if err != nil {
		// TODO: ignore 404
		// log it
		return nil, err
	}

	prpa := genPrpa(binInfo.Project, binInfo.Repository, binInfo.Arch)

	haveMeta := sets.NewString()
	newCaches := []cacheBin{}

	for i := range res {
		name := res[i].Name

		if bin, _, ok := isBinFile(name); ok {
			tmp := filepath.Join(dir, name)
			stat, _ := os.Stat(tmp)
			md5, _ := utils.GenMd5OfFile(tmp)

			newCaches = append(newCaches, cacheBin{
				cacheBinInfo: cacheBinInfo{
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
