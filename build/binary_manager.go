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
		dir:         dir,
		buildHelper: b.buildHelper,
	}
	h.init(repo)

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

	nometa     bool
	dir        string
	prpa       string
	project    string
	repository string
	repoServer string

	binaries map[string]binaryInfo
}

func (h *binaryManagerHelper) init(repo *RepoPath) {
	info := h.getBuildInfo()
	h.prpa = info.getPrpaOfRepo(repo)
	h.nometa = info.isRepoNoMeta(repo)
	h.repoServer = info.getRepoServer(repo)

	h.project = repo.Project
	h.repository = repo.Repository

	h.binaries = make(map[string]binaryInfo)
}

func (h *binaryManagerHelper) genCommonOpts() binary.CommonOpts {
	info := h.getBuildInfo()

	return binary.CommonOpts{
		WorkerId:   h.cfg.Id,
		Project:    h.project,
		Repository: h.repository,
		Arch:       info.Arch,
		Modules:    info.Modules,
	}
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
	opts := h.genCommonOpts()
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
		utils.LogErr("getbinaryversions, err: %s", err.Error())
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
		bv := h.toBinaryMap(v.Binaries)

		for _, k := range bins {
			if _, ok := bv[k]; !ok {
				return nil
			}
		}

		return bv
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

	if len(bv) == 0 {
		toDownload = bins

		return
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

		useCache, haveMeta, binFile := h.checkInCache(binName, bin)

		if !useCache {
			toDownload = append(toDownload, binName)
			size += bin.SizeK
		} else {
			if stat, err := os.Stat(binFile); err != nil {
				utils.LogErr("stat file:%s, err:%v\n", binFile, err)
			} else {
				oldCache = append(oldCache, cacheBinInfo{
					Id:   genCacheId(h.prpa, bin.HdrMD5),
					Size: int(stat.Size()),
				})
			}

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
	binFile string,
) {
	cacheDir := h.getCacheDir()
	if cacheDir == "" {
		return
	}
	cacheFile := genCacheFile(cacheDir, genCacheId(h.prpa, bin.HdrMD5))

	to := filepath.Join(h.dir, bin.Name)
	if nil != linkOrCopy(cacheFile, to) {
		return
	}

	useCache, haveMeta = h.checkMetaInCache(binName, cacheFile, bin)

	if useCache && queryHdrmd5(to) == bin.HdrMD5 {
		binFile = to

		return
	}

	useCache = false
	os.Remove(to)

	return
}

func (h *binaryManagerHelper) checkMetaInCache(binName, cacheFile string, bin *binary.Binary) (
	useCache bool,
	haveMeta bool,
) {
	if h.nometa || bin.HdrMD5 == "" {
		useCache = true

		return
	}

	tmp := filepath.Join(h.dir, binName+".meta")
	if nil != linkOrCopy(cacheFile+".meta", tmp) {
		return
	}

	if md5, err := utils.GenMd5OfFile(tmp); err != nil {
		utils.LogErr(
			"gen md5 of file: %s, err: %s",
			tmp, err.Error(),
		)
	} else {
		if md5 == bin.MetaMD5 {
			useCache = true
			haveMeta = true

			return
		}
	}

	os.Remove(tmp)

	return
}

func (h *binaryManagerHelper) download(toDownload []string) ([]cacheBin, error) {
	opts := binary.DownloadOpts{
		CommonOpts: h.genCommonOpts(),
	}
	opts.Binaries = toDownload

	res, err := binary.Download(h.gethc(), h.repoServer, &opts, h.dir)
	if err != nil {
		utils.LogErr("call api of getbinaries, err: %s", err)

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
				utils.LogErr("stat %s, err: %s", tmp, err.Error())

				continue
			}

			md5, err := utils.GenMd5OfFile(tmp)
			if err != nil {
				utils.LogErr("gen md5 of %s, err: %s", tmp, err.Error())

				continue
			}

			newCaches = append(newCaches, cacheBin{
				cacheBinInfo: cacheBinInfo{
					Id:   genCacheId(h.prpa, md5),
					Size: int(stat.Size()),
				},
				binFile: tmp,
			})

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
