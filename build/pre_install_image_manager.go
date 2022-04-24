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

	handleRepoBins func(prpa string, bins binary.BinaryVersionList)
}

func (h *preInstallImageManager) getPkgDir() string {
	return h.env.pkgdir
}

func (h *preInstallImageManager) getHttpClient() *utils.HttpClient {
	return &h.hc
}

func (h *preInstallImageManager) getPreInstallImage(bins []string) (pre preInstallImage) {
	info := h.info

	prpas := make(map[string][]string)
	metas := make(map[string]binary.Binary)
	hdrmd5s := make(map[string]string)
	imageOrigins := make(map[string]string)
	binsSet := sets.NewString(bins...)

	for _, repo := range info.Paths {
		v, endpoint, err := h.getBinary(&repo, binsSet.UnsortedList())
		if err != nil {
			// log it
			continue
		}

		prpa := genPrpa(repo.Project, repo.Repository, info.Arch)

		if h.handleRepoBins != nil {
			h.handleRepoBins(prpa, v)
		}

		if endpoint != info.SrcServer {
			prpas[endpoint] = append(prpas[endpoint], prpa)
		}

		nometa := h.isRepoNoMeta(&repo)
		items := v.Binaries
		for i := range items {
			item := &items[i]

			if bin, _, ok := isBinFile(item.Name); ok {
				hdrmd5s[bin] = item.HdrMD5
				imageOrigins[bin] = prpa
				binsSet.Delete(bin)

				if !nometa && item.MetaMD5 != "" {
					metas[bin] = *item
				}
			}
		}

		if binsSet.Len() == 0 {
			break
		}
	}

	if len(prpas) == 0 {
		return
	}

	for k, v := range hdrmd5s {
		if v == "" {
			delete(hdrmd5s, k)
		}
	}

	for _, item := range info.getAllNotInstallBDep() {
		delete(hdrmd5s, item.Name)
	}

	if len(hdrmd5s) == 0 {
		return
	}

	// ok, now check if there is an image on one of the repo servers

	images := h.getImagesFromRepo(hdrmd5s, prpas)

	imageList := []imageInfo{}
	for k, v := range images {
		for i := range v {
			imageList = append(imageList, imageInfo{&v[i], k})
		}
	}

	img := h.chooseBestImage(imageList, hdrmd5s, metas)
	pre.img = *img.img

	imagesWithMeta := sets.NewString()
	for k := range metas {
		imagesWithMeta.Insert(k)
	}
	pre.imagesWithMeta = imagesWithMeta

	return
}

func (h *preInstallImageManager) getBinary(repo *RepoPath, bins []string) (binary.BinaryVersionList, string, error) {
	info := h.info

	endpoint := repo.Server
	if endpoint == "" {
		endpoint = info.RepoServer
	}

	opts := binary.ListOpts{
		CommonOpts: binary.CommonOpts{
			Project:    repo.Project,
			Repository: repo.Repository,
			Arch:       info.Arch,
			Modules:    info.Modules,
			Binaries:   bins,
		},
		NoMeta: h.isRepoNoMeta(repo),
	}

	v, err := binary.List(h.getHttpClient(), endpoint, &opts)
	return v, endpoint, err
}

func (h *preInstallImageManager) isRepoNoMeta(repo *RepoPath) bool {
	info := h.info

	return repo.Project != info.Project ||
		repo.Repository != info.Repository ||
		info.isPreInstallImage()
}

func (h *preInstallImageManager) getImagesFromRepo(
	hdrmd5s map[string]string,
	prpas map[string][]string,
) map[string][]image.Image {
	// TODO: how to generate match
	match := make([]byte, 512)

	images := make(map[string][]image.Image)

	for endpoint, prpa := range prpas {
		v, err := image.Post(
			h.getHttpClient(), endpoint,
			&image.QueryOpts{
				Prpa: prpa,
			},
			match,
		)
		if err != nil {
			// log it
			continue
		}

		images[endpoint] = v
	}

	return images
}

func (h *preInstallImageManager) getPrpa() string {
	v := h.info

	return genPrpa(v.Project, v.Repository, v.Arch)
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

	prpa := h.getPrpa()
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

		if img.SizeK == "" || img.HdrMD5 == "" {
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

	ifile := img.getImageFilePath(h.getPkgDir())
	os.Remove(ifile)

	if nil != linkOrCopy(cacheFile, ifile) {
		return false
	}

	defer os.Remove(ifile)

	if !ismatch() {
		return false
	}

	manager := cacheManager{h.buildHelper}

	if v, err := os.Stat(ifile); err == nil {
		manager.pruneCache(
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
	manager := cacheManager{h.buildHelper}

	cacheDir := h.getCacheDir()
	if cacheDir != "" {
		// make room
		n, _ := strconv.Atoi(img.img.SizeK)
		manager.pruneCache(h.getCacheSize()-(n<<10), nil, nil)
	}

	ifile := img.getImageFilePath(h.getPkgDir())
	os.Remove(ifile)

	err := image.Download(h.getHttpClient(), img.loadFrom, img.img.Prpa, img.img.Path, ifile)
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
	if nil == writeFile(tmp, []byte(data)) {
		manager.pruneCache(
			h.getCacheSize(), nil,
			[]cacheBin{
				{
					cacheBinInfo: cacheBinInfo{
						cacheId:   img.genCacheId(),
						cacheSize: int(v.Size()),
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

	prpa := h.getPrpa()
	cacheDir := h.getCacheDir()
	if cacheDir != "" {
		todo = sets.NewString()

		for bin := range bins {
			bv := metas[bin]

			cacheId := genCacheId(prpa, bv.HdrMD5)
			cacheFile := genCacheFile(cacheDir, cacheId)

			// copy from cache
			tmp := filepath.Join(h.getPkgDir(), bin+".meta")
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
	info := h.info
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

	res, err := binary.Download(h.getHttpClient(), endpoint, &opts, h.getPkgDir())
	if err != nil {
		// log it
		return false
	}

	prpa := h.getPrpa()
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

		metaFile := filepath.Join(h.getPkgDir(), name)
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
