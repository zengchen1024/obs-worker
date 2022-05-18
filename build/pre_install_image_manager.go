package build

import (
	"os"
	"path/filepath"
	"strconv"

	"github.com/zengchen1024/obs-worker/sdk/binary"
	"github.com/zengchen1024/obs-worker/sdk/image"
	"github.com/zengchen1024/obs-worker/utils"
	"k8s.io/apimachinery/pkg/util/sets"
)

type preInstallImageManager struct {
	*buildHelper

	cache *cacheManager

	handleRepoBins func(prpa string, bins binary.BinaryVersionList)
}

func (h *preInstallImageManager) getPreInstallImage(bins []string) (pre preInstallImage) {
	prpas := make(map[string][]string)
	metas := make(map[string]binary.Binary)
	hdrmd5s := make(map[string]string)
	imageOrigins := make(map[string]string)

	binsSet := sets.NewString(bins...)
	info := h.getBuildInfo()
	for _, repo := range info.Paths {
		v, endpoint, err := h.getBinary(&repo, binsSet.UnsortedList())
		if err != nil {
			utils.LogErr("getbinaryversions, err:%s", err.Error())

			continue
		}

		if len(v.Binaries) == 0 {
			break
		}

		prpa := info.getPrpaOfRepo(&repo)

		if h.handleRepoBins != nil {
			h.handleRepoBins(prpa, v)
		}

		if endpoint != info.SrcServer {
			prpas[endpoint] = append(prpas[endpoint], prpa)
		}

		nometa := info.isRepoNoMeta(&repo)

		items := v.Binaries
		for i := range items {
			item := &items[i]

			if item.Error != "" {
				continue
			}

			if bin, _, ok := isBinFile(item.Name); ok {
				if !nometa && item.MetaMD5 != "" {
					metas[bin] = *item
				}

				if binsSet.Has(bin) {
					hdrmd5s[bin] = item.HdrMD5
					imageOrigins[bin] = prpa

					binsSet.Delete(bin)
				}
			}
		}

		if binsSet.Len() == 0 {
			break
		}
	}

	if len(prpas) == 0 {
		utils.LogErr("no prpas")

		return
	}

	for _, item := range info.getNoInstallBDep() {
		if _, ok := hdrmd5s[item.Name]; ok {
			delete(hdrmd5s, item.Name)
		}
	}

	if len(hdrmd5s) == 0 {
		utils.LogErr("no hdrmd5s")

		return
	}

	// Now, check if there is an image in one of the repo servers

	images := h.getImagesFromRepo(hdrmd5s, prpas)

	imageList := []imageInfo{}
	for k, v := range images {
		for i := range v {
			imageList = append(imageList, imageInfo{&v[i], k})
		}
	}

	img := h.chooseBestImage(imageList, hdrmd5s, metas)
	if img == nil {
		return
	}

	pre.img = *img.img

	imagesWithMeta := sets.NewString()
	for k := range metas {
		imagesWithMeta.Insert(k)
	}
	pre.imagesWithMeta = imagesWithMeta

	return
}

func (h *preInstallImageManager) getBinary(repo *RepoPath, bins []string) (
	binary.BinaryVersionList, string, error,
) {
	info := h.getBuildInfo()

	opts := binary.ListOpts{
		CommonOpts: binary.CommonOpts{
			Project:    repo.Project,
			Repository: repo.Repository,
			Arch:       info.Arch,
			Modules:    info.Modules,
			Binaries:   bins,
		},
		NoMeta: info.isRepoNoMeta(repo),
	}

	endpoint := info.getRepoServer(repo)

	v, err := binary.List(h.gethc(), endpoint, &opts)

	return v, endpoint, err
}

func (h *preInstallImageManager) getImagesFromRepo(
	hdrmd5s map[string]string,
	prpas map[string][]string,
) map[string][]image.Image {
	match := make([]byte, 512)
	for _, item := range hdrmd5s {
		offset, _ := strconv.ParseInt(item[0:3], 16, 32)
		i := offset >> 3
		p := offset & 0x7

		match[i] |= 1 << p
	}

	images := make(map[string][]image.Image)

	for endpoint, prpa := range prpas {
		v, err := image.Post(
			h.gethc(), endpoint,
			&image.QueryOpts{
				Prpa: prpa,
			},
			match, h.workDir,
		)
		if err != nil {
			utils.LogErr("getpreinstallimageinfos, err: %s", err.Error())

			continue
		}

		images[endpoint] = v
	}

	return images
}

func (h *preInstallImageManager) chooseBestImage(
	images []imageInfo,
	hdrmd5s map[string]string,
	metas map[string]binary.Binary,
) *imageInfo {
	neededHdrmd5s := sets.NewString()
	for _, v := range hdrmd5s {
		neededHdrmd5s.Insert(v)
	}

	prpa := h.getBuildInfo().getPrpa()

	var bestImage *imageInfo
	for {
		bestImage := h.findBestImage(images, neededHdrmd5s, prpa, bestImage)
		if bestImage == nil {
			break
		}

		if h.isImageInCache(bestImage) &&
			h.getImageMetas(bestImage, hdrmd5s, metas) {
			break
		}

		if h.downloadImage(bestImage) &&
			h.getImageMetas(bestImage, hdrmd5s, metas) {
			break
		}
	}

	return bestImage
}

func (h *preInstallImageManager) findBestImage(
	images []imageInfo,
	neededHdrmd5s sets.String,
	prpa string,
	oldOne *imageInfo,
) *imageInfo {
	// don't find this one again
	if oldOne != nil {
		oldOne.img.HdrMD5s = nil
	}

	info := h.info
	v := info.isPreInstallImage()

	bestImageNum := 2
	bestImage := -1

	for i := range images {
		img := images[i].img

		if len(img.HdrMD5s) < bestImageNum {
			continue
		}

		if img.SizeK == 0 || img.HdrMD5 == "" {
			continue
		}

		if img.Prpa == prpa && img.Package == info.Package {
			continue
		}

		if sets.NewString(img.HdrMD5s...).Difference(neededHdrmd5s).Len() > 0 {
			continue
		}

		if v && neededHdrmd5s.Difference(sets.NewString(img.HdrMD5s...)).Len() == 0 {
			continue
		}

		bestImage = i
		bestImageNum = len(img.HdrMD5s)
		//TODO: It seems that the newer best image will replace the previous one when
		// the current one's image num >= previous one's.
	}

	if bestImage >= 0 {
		return &images[bestImage]
	}

	return nil
}

func (h *preInstallImageManager) isImageInCache(img *imageInfo) bool {
	cacheDir := h.getCacheDir()
	if cacheDir == "" {
		return false
	}

	meta := img.genCacheMeta()
	cacheId := img.genCacheId()
	cacheFile := filepath.Join(cacheDir, cacheId[0:2], cacheId)

	ismatch := func() bool {
		b, err := os.ReadFile(cacheFile + ".meta")

		return err == nil && string(b) == meta
	}

	if !ismatch() {
		return false
	}

	ifile := img.getImageFilePath(h.getPkgdir())
	os.Remove(ifile)

	if nil != linkOrCopy(cacheFile, ifile) {
		return false
	}

	defer os.Remove(ifile)

	if !ismatch() {
		return false
	}

	if v, err := os.Stat(ifile); err == nil {
		h.cache.pruneCache(
			h.getCacheSize(),
			[]cacheBinInfo{
				{cacheId, int(v.Size())},
			},
			nil,
		)
	}

	return true
}

func (h *preInstallImageManager) downloadImage(img *imageInfo) bool {
	cacheDir := h.getCacheDir()
	if cacheDir != "" {
		h.cache.pruneCache(h.getCacheSize()-(img.img.SizeK<<10), nil, nil)
	}

	ifile := img.getImageFilePath(h.getPkgdir())
	os.Remove(ifile)

	err := image.Download(h.gethc(), img.loadFrom, img.img.Prpa, img.img.Path, ifile)
	if err != nil {
		return false
	}

	defer os.Remove(ifile)

	v, err := os.Stat(ifile)
	if err != nil || v.Size() == 0 {
		return false
	}

	// manage_cache
	data := img.genCacheMeta()
	tmp := ifile + ".meta"
	if nil == utils.WriteFile(tmp, []byte(data)) {
		h.cache.pruneCache(
			h.getCacheSize(), nil,
			[]cacheBin{
				{
					cacheBinInfo: cacheBinInfo{
						Id:   img.genCacheId(),
						Size: int(v.Size()),
					},
					binFile: ifile,
				},
			},
		)

		os.Remove(tmp)
	}

	return true
}

func (h *preInstallImageManager) getImageMetas(
	img *imageInfo,
	hdrmd5s map[string]string,
	metas map[string]binary.Binary,
) bool {
	knownHdrmd5s := sets.NewString(img.img.HdrMD5s...)

	bins := sets.NewString()
	for k := range metas {
		if knownHdrmd5s.Has(hdrmd5s[k]) {
			bins.Insert(k)
		}
	}

	todo := bins

	prpa := h.getBuildInfo().getPrpa()
	cacheDir := h.getCacheDir()
	if cacheDir != "" {
		todo = sets.NewString()

		for bin := range bins {
			bv := metas[bin]

			cacheId := genCacheId(prpa, bv.HdrMD5)
			cacheFile := genCacheFile(cacheDir, cacheId)

			// copy from cache
			tmp := filepath.Join(h.getPkgdir(), bin+".meta")
			if nil == linkOrCopy(cacheFile+".meta", tmp) {
				v, err := utils.GenMd5OfFile(tmp)
				if err == nil && v == bv.HdrMD5 {
					continue
				}
				os.Remove(tmp)
			}

			todo.Insert(bin)
		}
	}

	if todo.Len() == 0 {
		return true
	}

	return h.downloadImageMeta(todo, metas)
}

func (h *preInstallImageManager) downloadImageMeta(
	todo sets.String,
	metas map[string]binary.Binary,
) bool {
	info := h.getBuildInfo()

	opts := binary.DownloadOpts{
		CommonOpts: binary.CommonOpts{
			WorkerId:   h.getWorkerId(),
			Project:    info.Project,
			Repository: info.Repository,
			Arch:       info.Arch,
			Binaries:   todo.UnsortedList(),
		},
	}

	endpoint := info.RepoServer
	for _, v := range info.Paths {
		if v.Project == info.Project && v.Repository == info.Repository && v.Server != "" {
			endpoint = v.Server
			break
		}
	}

	res, err := binary.Download(h.gethc(), endpoint, &opts, h.getPkgdir())
	if err != nil {
		// log it
		return false
	}

	prpa := info.getPrpa()
	cacheDir := h.getCacheDir()

	done := sets.NewString()
	for i := range res {
		name := res[i].Name
		bin, ok := isMetaFile(name)
		if !ok {
			continue
		}

		bv, ok := metas[bin]
		if !ok {
			// log: downloaded the wrong meta
			return false
		}

		metaFile := filepath.Join(h.getPkgdir(), name)
		v, err := utils.GenMd5OfFile(metaFile)
		if err != nil {
			// log it
			continue
		}

		if v == bv.MetaMD5 {
			done.Insert(bin)

			if cacheDir != "" {
				cacheId := genCacheId(prpa, bv.HdrMD5)
				cacheFile := genCacheFile(cacheDir, cacheId)

				tmp := cacheFile + ".meta.$$"
				if nil == linkOrCopy(metaFile, tmp) {
					if nil != os.Rename(tmp, cacheFile+".meta") {
						// log it
						return false
					}
				}
			}

		} else {
			os.Remove(metaFile)
		}
	}

	return todo.Difference(done).Len() == 0
}

type imageInfo struct {
	img      *image.Image
	loadFrom string
}

func (b *imageInfo) genCacheId() string {
	return genCacheId(b.img.Prpa, b.img.HdrMD5)
}

func (b *imageInfo) genCacheMeta() string {
	return genMetaLine(b.img.HdrMD5, ":preinstallimage")
}

func (b *imageInfo) getImageFilePath(dir string) string {
	return filepath.Join(dir, getImageFile(b.img))
}
