package obsbuild

import (
	"io"
	"os"
	"strings"
)

var knownBins = []string{".rpm", ".deb", ".pkg.tar.gz", ".pkg.tar.xz", ".pkg.tar.zst"}

func isBinFile(name string) (string, string, bool) {
	for _, suffix := range knownBins {
		if strings.HasSuffix(name, suffix) {
			return strings.TrimSuffix(name, suffix), suffix, true
		}
	}

	return "", "", false
}

func isMetaFile(name string) (string, bool) {
	if s := ".meta"; strings.HasSuffix(name, s) {
		return strings.TrimSuffix(name, s), true
	}

	return "", false
}

func queryHdrmd5(file string) string {
	return ""
}

func linkOrCopy(src, dst string) (err error) {
	os.Remove(dst)

	if err = os.Link(src, dst); err == nil {
		return nil
	}

	in, err := os.Open(src)
	if err != nil {
		return
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return
	}
	defer func() {
		// don't override the previous error
		if err1 := out.Close(); err1 != nil {
			if err == nil {
				err = err1
			}

			os.Remove(dst)
		}
	}()

	if _, err = io.Copy(out, in); err != nil {
		return
	}

	err = out.Sync()

	return
}
