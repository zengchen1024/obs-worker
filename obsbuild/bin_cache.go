package obsbuild

import (
	"os"
	"path/filepath"

	"github.com/opensourceways/obs-worker/utils"
)

type binCache struct {
	binFile string

	binCacheInfo
}

func (c binCache) getBinMetaFile() string {
	if bin, _, ok := isBinFile(c.binFile); ok {
		return bin + ".meta"
	}

	return c.binFile + ".meta"
}

type binCacheInfo struct {
	cacheId   string
	cacheSize int
}

func (c binCacheInfo) getCacheFile(cacheDir string) string {
	return filepath.Join(cacheDir, c.cacheId[0:2], c.cacheId)
}

func (b *buildOnce) manageCache(pruneSize int, old []binCacheInfo, news []binCache) error {
	m := cacheManager{b}
	return m.pruneCache(pruneSize, old, news)
}

type cacheManager struct {
	b *buildOnce
}

// read from "/var/cache/obs/worker/cache/content"
// filter the items which does not have size
func (h *cacheManager) getCurrentCache(cf utils.FileOp) ([]binCacheInfo, error) {
	//TODO
	return nil, nil
}

func (h *cacheManager) addNewCache(caches []binCache) ([]binCacheInfo, error) {
	cacheDir := h.b.getCacheDir()

	r := []binCacheInfo{}

	for i := len(caches) - 1; i >= 0; i-- {
		c := &caches[i]

		// copy bin File
		cacheFile := c.getCacheFile(cacheDir)
		mkdirAll(filepath.Dir(cacheFile))

		tmp := cacheFile + ".$$"

		if err := linkOrCopy(c.binFile, tmp); err != nil {
			continue
		}

		if err := os.Rename(tmp, cacheFile); err != nil {
			os.Remove(tmp)
			return nil, err
		}

		// copy meta file
		cacheMetaFile := cacheFile + ".meta"
		os.Remove(cacheMetaFile)

		metaFile := c.getBinMetaFile()
		if b, err := isEmptyFile(metaFile); err == nil && !b {
			tmp = cacheMetaFile + ".$$"

			if err := linkOrCopy(metaFile, tmp); err == nil {
				if err = os.Rename(tmp, cacheMetaFile); err != nil {
					os.Remove(tmp)
					return nil, err
				}
			}
		}

		r = append(r, c.binCacheInfo)
	}

	return r, nil
}

func (h *cacheManager) pruneCache(pruneSize int, oldCache []binCacheInfo, news []binCache) error {
	cf, err := utils.LockOpen(
		filepath.Join(h.b.getCacheDir(), "content"),
		os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644,
	)
	if err != nil {
		return err
	}

	defer cf.Close()

	v1, err := h.addNewCache(news)
	if err != nil {
		return err
	}

	v, err := h.getCurrentCache(cf)
	if err != nil {
		return err
	}

	caches := make([]binCacheInfo, 0, len(oldCache)+len(v)+len(v1))
	caches = append(caches, v1...)
	caches = append(caches, oldCache...)
	caches = append(caches, v...)

	var i int
	for i = range caches {
		pruneSize -= caches[i].cacheSize
		if pruneSize < 0 {
			break
		}
	}

	cacheDir := h.b.getCacheDir()

	for j := i; j < len(caches); j++ {
		f := caches[j].getCacheFile(cacheDir)
		os.Remove(f)
		os.Remove(f + ".meta")
	}

	//save caches[0:i] to cacheDir/content (/var/cache/obs/worker/cache/content)
	// if save failed, delete all new caches

	for j := range v1 {
		f := v1[j].getCacheFile(cacheDir)
		os.Remove(f)
		os.Remove(f + ".meta")
	}

	return nil
}
