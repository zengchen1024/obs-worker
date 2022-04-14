package obsbuild

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/opensourceways/obs-worker/sdk/binary"
	"github.com/opensourceways/obs-worker/sdk/image"
	"github.com/opensourceways/obs-worker/utils"
	"k8s.io/apimachinery/pkg/util/sets"
)

func genPrpa(proj, repo, arch string) string {
	return fmt.Sprintf("%s/%s/%s", proj, repo, arch)
}

func pasePrps(s string) (string, string, string) {
	if v := strings.Split(s, "/"); len(v) == 3 {
		return v[0], v[1], v[2]
	}

	return s, "", ""
}

func genCacheId(prpa, hdrmd5 string) string {
	return utils.GenMD5([]byte(fmt.Sprintf("%s/%s", prpa, hdrmd5)))
}

func genCacheFile(cacheDir, cacheId string) string {
	return filepath.Join(cacheDir, cacheId[0:2], cacheId)
}

func getImageFile(img *image.Image) string {
	return "preinstallimage." + img.File
}

type preInstallImage struct {
	hdrmd5s map[string]string

	img image.Image

	imagesWithMeta sets.String
	imageOrigins   map[string]string
}

func (p *preInstallImage) isEmpty() bool {
	return p.img.File == ""
}

func (p *preInstallImage) getImageName() string {
	return getImageFile(&p.img)
}

func (p *preInstallImage) getImageBins() (map[string]string, sets.String, map[string]string) {
	knownHdrmd5s := sets.NewString(p.img.HdrMD5s...)

	metas := sets.NewString()
	bins := make(map[string]string)
	imageToPrpa := make(map[string]string)
	for k, v := range p.hdrmd5s {
		if knownHdrmd5s.Has(v) {
			bins[k] = v

			if p.imagesWithMeta.Has(k) {
				metas.Insert(k)
			}

			if v1, ok := p.imageOrigins[k]; ok {
				imageToPrpa[k] = v1
			}
		}
	}

	return bins, metas, imageToPrpa
}

func (p *preInstallImage) getImageSource() string {
	prpa := p.img.Prpa

	// strip arch
	s := prpa
	if i := strings.LastIndex(prpa, "/"); i > 0 {
		s = prpa[:i]
	}

	if p.img.Package != "" {
		s = fmt.Sprintf("%s/%s", s, p.img.Package)
	}

	return fmt.Sprintf("%s %s", s, p.img.HdrMD5)
}

func (b *buildOnce) getPreInstallImage(bins []string) (pre preInstallImage) {
	info := b.info

	prpas := make(map[string][]string)
	bvls := make(map[string]binary.BinaryVersionList)
	metas := make(map[string]binary.Binary)
	hdrmd5s := make(map[string]string)
	imageOrigins := make(map[string]string)
	binsSet := sets.NewString(bins...)
	helper := &preinstallImageHelper{b}

	for _, repo := range info.Path {
		v, endpoint, err := helper.getBinary(&repo, binsSet.UnsortedList())
		if err != nil {
			// log it
			continue
		}

		prpa := genPrpa(repo.Project, repo.Repository, info.Arch)

		bvls[prpa] = v
		if endpoint != info.SrcServer {
			prpas[endpoint] = append(prpas[endpoint], prpa)
		}

		nometa := helper.isRepoNoMeta(&repo)
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

	images := helper.getImagesFromRepo(hdrmd5s, prpas)

	imageList := []imageInfo{}
	for k, v := range images {
		for i := range v {
			imageList = append(imageList, imageInfo{&v[i], k})
		}
	}

	img := helper.chooseBestImage(imageList, hdrmd5s, metas)
	pre.img = *img.img

	imagesWithMeta := sets.NewString()
	for k := range metas {
		imagesWithMeta.Insert(k)
	}
	pre.imagesWithMeta = imagesWithMeta

	return
}

type preinstallImageHelper struct {
	b *buildOnce
}

func (h *preinstallImageHelper) getBinary(repo *RepoPath, bins []string) (binary.BinaryVersionList, string, error) {
	info := h.b.info

	endpoint := repo.Server
	if endpoint == "" {
		endpoint = info.RepoServer
	}

	opts := binary.ListOpts{
		Project:    repo.Project,
		Repository: repo.Repository,
		Arch:       info.Arch,
		Modules:    info.Module,
		Binaries:   bins,
		NoMeta:     h.isRepoNoMeta(repo),
	}

	v, err := binary.List(&h.b.hc, endpoint, &opts)
	return v, endpoint, err
}

func (h *preinstallImageHelper) isRepoNoMeta(repo *RepoPath) bool {
	info := h.b.info

	return repo.Project != info.Project ||
		repo.Repository != info.Repository ||
		info.isPreInstallImage()
}

func (h *preinstallImageHelper) getImagesFromRepo(
	hdrmd5s map[string]string,
	prpas map[string][]string,
) map[string][]image.Image {
	// TODO: how to generate match
	match := make([]byte, 512)

	images := make(map[string][]image.Image)

	for endpoint, prpa := range prpas {
		v, err := image.Post(
			&h.b.hc, endpoint,
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

func (h *preinstallImageHelper) getPrpa() string {
	v := h.b.info

	return genPrpa(v.Project, v.Repository, v.Arch)
}

func (h *preinstallImageHelper) chooseBestImage(
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

func (h *preinstallImageHelper) findBestImage(
	images []imageInfo,
	neededHdrmd5s sets.String,
	prpa string,
	oldOne *imageInfo,
) *imageInfo {
	// don't find this one again
	if oldOne != nil {
		oldOne.img.HdrMD5s = nil
	}

	info := h.b.info
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

func (h *preinstallImageHelper) getPkgDir() string {
	return h.b.env.pkgdir
}

func (h *preinstallImageHelper) isImageInCache(img *imageInfo) bool {
	cacheDir := h.b.getCacheDir()
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

	ifile := img.getImageFilePath(h.b.env.pkgdir)
	os.Remove(ifile)

	if nil != linkOrCopy(cacheFile, ifile) {
		return false
	}

	defer os.Remove(ifile)

	if !ismatch() {
		return false
	}

	if v, err := os.Stat(ifile); err == nil {
		h.b.manageCache(
			h.b.getCacheSize(),
			[]binCacheInfo{
				{cacheId, int(v.Size())},
			},
			nil,
		)
	}

	return true
}

func (h *preinstallImageHelper) downloadImage(img *imageInfo) bool {
	cacheDir := h.b.getCacheDir()
	if cacheDir != "" {
		// make room
		n, _ := strconv.Atoi(img.img.SizeK)
		h.b.manageCache(h.b.opts.cacheSize-(n<<10), nil, nil)
	}

	ifile := img.getImageFilePath(h.getPkgDir())
	os.Remove(ifile)

	err := image.Download(&h.b.hc, img.loadFrom, img.img.Prpa, img.img.Path, ifile)
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
	if nil == os.WriteFile(tmp, []byte(data), 0644) {
		h.b.manageCache(
			h.b.getCacheSize(), nil,
			[]binCache{
				{
					binCacheInfo: binCacheInfo{
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

func (h *preinstallImageHelper) getImageMetas(
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
	cacheDir := h.b.getCacheDir()
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

func (h *preinstallImageHelper) downloadImageMeta(
	todo sets.String,
	metas map[string]binary.Binary,
) bool {
	info := h.b.info
	opts := binary.DownloadOpts{
		WorkerId:   h.b.getWorkerId(),
		Project:    info.Project,
		Repository: info.Repository,
		Arch:       info.Arch,
		Binaries:   todo.UnsortedList(),
	}

	endpoint := info.RepoServer
	for _, v := range info.Path {
		if v.Project == info.Project && v.Repository == info.Repository && v.Server != "" {
			endpoint = v.Server
			break
		}
	}

	res, err := binary.Download(&h.b.hc, endpoint, &opts, h.getPkgDir())
	if err != nil {
		// log it
		return false
	}

	prpa := h.getPrpa()
	cacheDir := h.b.getCacheDir()

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
