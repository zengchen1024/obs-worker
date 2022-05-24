package build

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"sigs.k8s.io/yaml"

	"github.com/zengchen1024/obs-worker/utils"
)

func genCacheId(prpa, hdrmd5 string) string {
	return utils.GenMD5([]byte(
		fmt.Sprintf("%s/%s", prpa, hdrmd5),
	))
}

func genCacheFile(cacheDir, Id string) string {
	return filepath.Join(cacheDir, Id[0:2], Id)
}

type cacheBin struct {
	binFile string

	cacheBinInfo
}

func (c *cacheBin) getBinMetaFile() string {
	if bin, _, ok := isBinFile(c.binFile); ok {
		return bin + ".meta"
	}

	return c.binFile + ".meta"
}

type cacheBinInfo struct {
	Id   string `json:"id"`
	Size int    `json:"size"`
}

func (c *cacheBinInfo) toString() string {
	return fmt.Sprintf("%s %d", c.Id, c.Size)
}

func (c *cacheBinInfo) getCacheFile(cacheDir string) string {
	return genCacheFile(cacheDir, c.Id)
}

func (c *cacheBinInfo) remove(cacheDir string) {
	f := c.getCacheFile(cacheDir)
	os.Remove(f)
	os.Remove(f + ".meta")
}

type cacheManager struct {
	*buildHelper

	perlScriptDir    string
	parseCacheScript string
	setCacheScript   string
}

func (c *cacheManager) init(h *buildHelper) {
	c.buildHelper = h

	dir := filepath.Join(h.workDir, "perl")

	c.perlScriptDir = dir
	c.parseCacheScript = filepath.Join(dir, "parse_cache_content.pm")
	c.setCacheScript = filepath.Join(dir, "store_cache_content.pm")
}

func (c *cacheManager) getCurrentCacheInfo() ([]cacheBinInfo, error) {
	cache := struct {
		Content []cacheBinInfo `json:"content"`
	}{}

	v, err, _ := utils.RunCmd(
		"perl",
		"-I", c.perlScriptDir,
		c.parseCacheScript, c.getCacheDir(),
	)
	if err != nil {
		return nil, fmt.Errorf("%s, %v", v, err)
	}

	if err = yaml.Unmarshal(v, &cache); err != nil {
		return nil, err
	}

	return cache.Content, nil
}

func (c *cacheManager) setCacheInfo(cache []cacheBinInfo) error {
	s := make([]string, 0, len(cache))
	for _, item := range cache {
		if item.Size > 0 {
			s = append(s, item.toString())
		}
	}

	v, err, _ := utils.RunCmd(
		"perl",
		"-I", c.perlScriptDir,
		c.setCacheScript, c.getCacheDir(), strings.Join(s, "\n"),
	)
	if err != nil {
		return fmt.Errorf("%s, %v", v, err)
	}

	return nil
}

func (c *cacheManager) addNewCache(caches []cacheBin) ([]cacheBinInfo, error) {
	cacheDir := c.getCacheDir()
	r := []cacheBinInfo{}

	for i := len(caches) - 1; i >= 0; i-- {
		c := &caches[i]

		// copy bin File
		cacheFile := c.getCacheFile(cacheDir)

		mkdirAll(filepath.Dir(cacheFile))

		tmp := cacheFile + ".$$"

		if err := linkOrCopy(c.binFile, tmp); err != nil {
			utils.LogErr("copy %s to %s, err:%v\n", c.binFile, tmp, err)

			continue
		}

		if err := os.Rename(tmp, cacheFile); err != nil {
			os.Remove(tmp)
			utils.LogErr("rename %s to %s, err:%v\n", tmp, cacheFile, err)

			return nil, err
		}

		// copy meta file
		cacheFileMeta := cacheFile + ".meta"
		os.Remove(cacheFileMeta)

		metaFile := c.getBinMetaFile()
		if b, err := isEmptyFile(metaFile); err == nil && !b {
			tmp = cacheFileMeta + ".$$"

			if err := linkOrCopy(metaFile, tmp); err == nil {
				if err = os.Rename(tmp, cacheFileMeta); err != nil {
					os.Remove(tmp)

					return nil, err
				}
			}
		}

		r = append(r, c.cacheBinInfo)
	}

	return r, nil
}

func (c *cacheManager) pruneCache(pruneSize int, oldCache []cacheBinInfo, news []cacheBin) error {
	// if using content file itself as the lock file,
	// the later actions will unlock it such as renaming which
	// should be done in the context of locking.
	cf, err := utils.LockOpen(
		filepath.Join(c.getCacheDir(), "lock"),
		os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644,
	)
	if err != nil {
		return err
	}

	defer cf.Close()

	v1, err := c.addNewCache(news)
	if err != nil {
		return err
	}

	v, err := c.getCurrentCacheInfo()
	if err != nil {
		return err
	}

	caches := make([]cacheBinInfo, 0, len(oldCache)+len(v)+len(v1))
	caches = append(caches, v1...)
	caches = append(caches, oldCache...)
	caches = append(caches, v...)

	var i int
	for i = range caches {
		pruneSize -= caches[i].Size
		if pruneSize < 0 {
			break
		}
	}

	cacheDir := c.getCacheDir()

	n := len(caches)
	for j := i; j < n; j++ {
		caches[j].remove(cacheDir)
	}

	if err := c.setCacheInfo(caches[0:i]); err != nil {
		for j := range v1 {
			caches[j].remove(cacheDir)
		}

		return err
	}

	return nil
}
