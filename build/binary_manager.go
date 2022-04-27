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

	cache *cacheManager

	knowns map[string]binary.BinaryVersionList

	handleCacheHits       func(int)
	handleDownloadDetails func(num, size int)
}

func (b *binaryManager) init() {
	b.knowns = make(map[string]binary.BinaryVersionList)
}

func (b *binaryManager) setKnownBins(prpa string, bins binary.BinaryVersionList) {
	b.knowns[prpa] = bins
}

func (b *binaryManager) get(dir string, repo *RepoPath, bins []string) (map[string]binaryInfo, error) {
	h := &binaryManagerHelper{
		buildHelper: b.buildHelper,
	}
	h.init(dir, repo)

	oldCache, toDownload, total := h.getDownloads(bins, b.knowns)

	if b.handleCacheHits != nil {
		b.handleCacheHits(len(oldCache))
	}

	cacheDir := b.getCacheDir()

	var newCaches []cacheBin

	if n := len(toDownload); n > 0 {
		if b.handleDownloadDetails != nil {
			b.handleDownloadDetails(n, total)
		}

		if cacheDir != "" {
			cacheSize := b.getCacheSize()

			if v := total << 10; v*100 > cacheSize {
				b.cache.pruneCache(cacheSize-v, nil, nil)
			}
		}

		var err error
		newCaches, err = h.download(toDownload)
		if err != nil {
			return nil, err
		}
	}

	if cacheDir != "" {
		b.cache.pruneCache(b.getCacheSize(), oldCache, newCaches)
	}

	return h.getBinaries(), nil
}

type binaryManagerHelper struct {
	*buildHelper

	commonOpts binary.CommonOpts
	dir        string
	prpa       string
	repoServer string
	nometa     bool

	binaries map[string]binaryInfo
}

func (h *binaryManagerHelper) init(dir string, repo *RepoPath) {
	info := h.getBuildInfo()

	h.commonOpts = binary.CommonOpts{
		WorkerId:   h.cfg.Id,
		Project:    repo.Project,
		Repository: repo.Repository,
		Arch:       info.Arch,
		Modules:    info.Modules,
	}

	h.dir = dir
	h.prpa = info.getPrpaOfRepo(repo)
	h.nometa = info.isRepoNoMeta(repo)
	h.binaries = make(map[string]binaryInfo)
	h.repoServer = info.getRepoServer(repo)
}

func (h *binaryManagerHelper) getBinaries() map[string]binaryInfo {
	if !h.nometa {
		return h.binaries
	}

	ret := h.binaries

	for k, v := range ret {
		if v.hasMeta {
			os.Remove(filepath.Join(h.dir, k+".meta"))

			v.hasMeta = false
			ret[k] = v
		}
	}

	return ret
}

func (h *binaryManagerHelper) toBinaryMap(bl []binary.Binary) map[string]*binary.Binary {
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

func (h *binaryManagerHelper) listBinaries(bins []string) (binary.BinaryVersionList, error) {
	opts := h.commonOpts
	opts.Binaries = bins

	v, err := binary.List(
		h.gethc(),
		h.repoServer,
		&binary.ListOpts{
			CommonOpts: opts,
			NoMeta:     h.nometa,
		},
	)

	if err != nil {
		utils.LogErr("getbinaryversions, err:%v\n", err)
	}

	return v, err
}

func (h *binaryManagerHelper) checkByKnowns(
	bins []string,
	knowns map[string]binary.BinaryVersionList,
) map[string]*binary.Binary {
	if h.getCacheDir() == "" {
		return nil
	}

	if v, ok := knowns[h.prpa]; ok {
		hasAll := true
		bv := h.toBinaryMap(v.Binaries)

		for _, k := range bins {
			if _, ok := bv[k]; !ok {
				hasAll = false
				break
			}
		}

		if hasAll {
			return bv
		}
	}

	return nil
}

func (h *binaryManagerHelper) getDownloads(
	bins []string,
	knowns map[string]binary.BinaryVersionList,
) (
	oldCache []cacheBinInfo,
	toDownload []string,
	size int,
) {
	cacheDir := h.getCacheDir()

	bv := h.checkByKnowns(bins, knowns)
	if bv == nil && cacheDir != "" {
		if v, err := h.listBinaries(bins); err == nil {
			bv = h.toBinaryMap(v.Binaries)
		}
	}

	for _, binName := range bins {
		bin, ok := bv[binName]
		if !ok {
			toDownload = append(toDownload, binName)
			continue
		}
		if bin.Error != "" {
			continue
		}

		useCache, haveMeta, cache := h.checkInCache(binName, bin)

		if !useCache {
			toDownload = append(toDownload, binName)
			size += bin.SizeK
		} else {
			oldCache = append(oldCache, cache)

			h.binaries[binName] = binaryInfo{
				name:    bin.Name,
				hdrmd5:  bin.HdrMD5,
				hasMeta: haveMeta,
			}
		}
	}

	return
}

func (h *binaryManagerHelper) checkInCache(binName string, bin *binary.Binary) (
	useCache bool,
	haveMeta bool,
	cache cacheBinInfo,
) {
	cacheDir := h.getCacheDir()
	if cacheDir == "" {
		return
	}

	cacheId := genCacheId(h.prpa, bin.HdrMD5)
	cacheFile := genCacheFile(cacheDir, cacheId)

	to := filepath.Join(h.dir, bin.Name)
	if nil != linkOrCopy(cacheFile, to) {
		return
	}

	useCache = true

	if !h.nometa && bin.HdrMD5 != "" {
		tmp := filepath.Join(h.dir, binName+".meta")

		if nil == linkOrCopy(cacheFile+".meta", tmp) {
			md5, err := utils.GenMd5OfFile(tmp)
			if err != nil {
				utils.LogErr("gen md5 of file:%s, err:%v\n", tmp, err)
			}

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
		stat, err := os.Stat(to)
		if err != nil {
			utils.LogErr("stat file:%s, err:%v\n", to, err)
		} else {
			cache = cacheBinInfo{cacheId, int(stat.Size())}
		}

		return
	}

	useCache = false
	os.Remove(to)

	return
}

func (h *binaryManagerHelper) download(toDownload []string) ([]cacheBin, error) {
	opts := binary.DownloadOpts{
		CommonOpts: h.commonOpts,
	}
	opts.Binaries = toDownload

	res, err := binary.Download(h.gethc(), h.repoServer, &opts, h.dir)
	if err != nil {
		utils.LogErr("call api of getbinaries, err:%v\n", err)
		return nil, err
	}

	haveMeta := sets.NewString()
	newCaches := []cacheBin{}

	for i := range res {
		name := res[i].Name

		if bin, _, ok := isBinFile(name); ok {
			tmp := filepath.Join(h.dir, name)

			stat, err := os.Stat(tmp)
			if err != nil {
				utils.LogErr("stat %s, err:%v\n", tmp, err)

				continue
			}

			md5, err := utils.GenMd5OfFile(tmp)
			if err != nil {
				utils.LogErr("gen md5 of %s, err:%v\n", tmp, err)

				continue
			}

			newCaches = append(
				newCaches,
				cacheBin{
					cacheBinInfo: cacheBinInfo{
						Id:   genCacheId(h.prpa, md5),
						Size: int(stat.Size()),
					},
					binFile: tmp,
				},
			)

			h.binaries[bin] = binaryInfo{
				name:   name,
				hdrmd5: md5,
			}

		} else if bin, ok := isMetaFile(name); ok {
			haveMeta.Insert(bin)
		}
	}

	for k := range haveMeta {
		if v, ok := h.binaries[k]; ok {
			v.hasMeta = true
			h.binaries[k] = v
		}
	}

	return newCaches, nil
}
