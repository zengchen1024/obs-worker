package build

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/cavaliergopher/rpm"

	"github.com/zengchen1024/obs-worker/utils"
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
	v, err := rpm.Open(file)
	if err != nil {
		utils.LogErr("get hdrmd5 of rpm file:%s, err:%v\n", file, err)
		return ""
	}

	if t := v.Signature.GetTag(0x03ec); t != nil {
		return fmt.Sprintf("%x", t.Bytes())
	}

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

func genMetaLine(md5, pkg string) string {
	return fmt.Sprintf("%s  %s", md5, pkg)
}

func splitMetaLine(line string) (string, string) {
	v := strings.Split(line, "  ")
	if len(v) == 2 {
		return v[0], v[1]
	}

	return line, ""
}

func isFileExist(f string) bool {
	_, err := os.Stat(f)
	return err == nil
}

func isEmptyFile(f string) (bool, error) {
	v, err := os.Stat(f)
	if err != nil {
		if os.IsNotExist(err) {
			return true, nil
		}

		return false, err
	}

	if v.IsDir() {
		return false, fmt.Errorf("%s is a directory", f)
	}

	return v.Size() == 0, nil
}

func readFileLineByLine(filename string, handle func(string) bool) error {
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		if b := handle(scanner.Text()); b {
			break
		}
	}

	return nil
}

func writeFile(f string, data []byte) error {
	return os.WriteFile(f, data, 0644)
}

func mkdir(dir string) error {
	return os.Mkdir(dir, os.FileMode(0777))
}

func mkdirAll(dir string) error {
	return os.MkdirAll(dir, os.FileMode(0777))
}

func cleanDir(dir string) {
	// TODO: refactor it
	d, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	for _, item := range d {
		os.RemoveAll(filepath.Join(dir, item.Name()))
	}
}
