package build

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/zengchen1024/obs-worker/sdk/buildinfo"
	"github.com/zengchen1024/obs-worker/sdk/job"
	"github.com/zengchen1024/obs-worker/utils"
)

type nonModeBuid struct {
	buildHelper

	imageManager  preInstallImageManager
	binaryManager binaryManager
	binaryLoader  nonModeBinary
	report        buildReport
	out           buildInfoOut
	rpmlist       buildRpmlist
	stats         buildStats
	build         buildPkg
	sources       buildSources
	cache         cacheManager
	oldpkg        buildOldPackages

	needOBSPackage bool
}

func newNonModeBuild(workDir string, cfg *Config, info *buildinfo.BuildInfo) (*nonModeBuid, error) {
	b := nonModeBuid{
		buildHelper: buildHelper{
			cfg:     cfg,
			info:    BuildInfo{BuildInfo: *info},
			workDir: workDir,
		},
	}

	h := &b.buildHelper

	b.sources = buildSources{h}

	b.rpmlist = buildRpmlist{
		buildHelper: h,
	}

	b.build = buildPkg{
		buildHelper: h,
	}

	b.cache = cacheManager{}
	b.cache.init(h)

	b.oldpkg = buildOldPackages{
		buildHelper:           h,
		handleDownloadDetails: b.stats.setBinaryDownloadDetail,
	}

	b.binaryManager = binaryManager{
		buildHelper:           h,
		cache:                 &b.cache,
		handleCacheHits:       b.stats.setCacheHit,
		handleDownloadDetails: b.stats.setBinaryDownloadDetail,
	}
	b.binaryManager.init()

	b.imageManager = preInstallImageManager{
		buildHelper:    h,
		cache:          &b.cache,
		handleRepoBins: b.binaryManager.setKnownBins,
	}

	b.binaryLoader = nonModeBinary{
		buildHelper:   h,
		imageManager:  &b.imageManager,
		binaryManager: &b.binaryManager,

		handleOutBDep:    b.out.setBdep,
		handleKiwiOrigin: b.report.setKiwiOrigin,

		handleImage: func(img *preInstallImage) {
			b.rpmlist.setPreInstallImage(img)
			b.stats.setPreInstallImage(img)
		},
	}

	return &b, nil
}

func (b *nonModeBuid) DoBuild(jobId string) error {
	if err := b.env.init(b.cfg); err != nil {
		return err
	}

	b.setBuildInfoOut()

	b.stats.recordDownloadStartTime()

	if err := b.fetchSources(); err != nil {
		return err
	}

	info := b.getBuildInfo()
	if !info.isNoUnchanged() && info.File != "preinstallimage" {
		if err := b.oldpkg.download(); err != nil {
			return err
		}
	}

	b.stats.recordDownloadTime()

	if err := b.downloadProjectConfig(); err != nil {
		return err
	}

	if err := b.rpmlist.generate(); err != nil {
		return err
	}

	_, err := b.build.do()
	if err != nil {
		return err
	}

	dir := b.env.otherDir

	mkdirAll(dir)

	b.stats.do(dir)

	b.out.writeBuildEnv(dir)

	b.report.do(dir)

	b.postBuild(jobId)

	return nil
}

func (b *nonModeBuid) setBuildInfoOut() {
	/*
	  if (!$kiwimode && !$followupmode && !$deltamode) {
	    $buildinfo->{'outbuildinfo'} = {
	      'project' => $projid,
	      'package' => $packid,
	      'repository' => $repoid,
	      'arch' => $arch,
	      'srcmd5' => $buildinfo->{'srcmd5'},
	      'verifymd5' => $buildinfo->{'verifymd5'} || $buildinfo->{'srcmd5'},
	      'bdep' => [],
	    };
	    for ('versrel', 'bcnt', 'release', 'module') {
	      $buildinfo->{'outbuildinfo'}->{$_} = $buildinfo->{$_} if defined $buildinfo->{$_};
	    }
	  }
	*/
}

func (b *nonModeBuid) fetchSources() error {
	s, err := b.sources.getSource()
	if err != nil {
		return err
	}

	metas := []string{s}

	needSSLCert, ignoreImage, _ := b.parseBuildFile()

	if needSSLCert {
		if err := b.sources.downloadSSLCert(); err != nil {
			return err
		}
	}

	if ignoreImage {
		b.report.collectOrigins = true
	}

	v, err := b.binaryLoader.getBinaries(!ignoreImage)
	if err != nil {
		return err
	}
	metas = append(metas, v...)

	return utils.WriteFile(b.env.meta, []byte(strings.Join(metas, "\n")+"\n"))
}

func (b *nonModeBuid) parseBuildFile() (
	needSSLCert bool,
	ignoreImage bool,
	needAppxSSLCert bool,
) {
	needOBSPackage := false

	re0 := regexp.MustCompile("^#\\s*needsbinariesforbuild\\s*$")
	re1 := regexp.MustCompile("^#\\s*needssslcertforbuild\\s*$|^Obs:\\s*needssslcertforbuild\\s*$")
	re2 := regexp.MustCompile("^(?:#|Obs:)\\s*needsappxsslcertforbuild\\s*$")

	filename := filepath.Join(b.getSrcdir(), b.getBuildInfo().File)

	readFileLineByLine(filename, func(line string) bool {
		bs := []byte(line)

		if re0.Match(bs) {
			ignoreImage = true
		}

		if re1.Match(bs) {
			needSSLCert = true
		}

		if re2.Match(bs) {
			needAppxSSLCert = true
		}

		if strings.Contains(line, "@OBS_PACKAGE@") {
			needOBSPackage = true
		}

		return ignoreImage && needSSLCert && needAppxSSLCert && needOBSPackage
	})

	if needOBSPackage {
		b.build.needOBSPackage = true
	}

	return
}

func (b *nonModeBuid) GetBuildInfo() *buildinfo.BuildInfo {
	return &b.info.BuildInfo
}

func (b *nonModeBuid) Kill()                  {}
func (b *nonModeBuid) SetSysrq()              {}
func (b *nonModeBuid) AppenBuildLog(s string) {}
func (b *nonModeBuid) GetBuildLogFile() string {
	return b.env.logFile
}

func (b *nonModeBuid) postBuild(jobId string) {
	info := b.getBuildInfo()

	opt := job.Opts{
		Job:      info.Job,
		Arch:     info.Arch,
		JobId:    jobId,
		Code:     "succeeded",
		WorkerId: b.cfg.Id,
	}

	files := b.listBuildResultFiles()
	if len(files) == 0 {
		opt.Code = "failed"
	}

	files = append(files,
		job.File{
			Name: "meta",
			Path: b.env.meta,
		},
		job.File{
			Name: "logfile",
			Path: b.env.logFile,
		},
	)

	err := job.Put(b.gethc(), info.RepoServer, opt, files)
	if err != nil {
		utils.LogErr("upload build files, err:%s", err.Error())
	}
}

func (b *nonModeBuid) listBuildResultFiles() []job.File {
	dir := b.env.packages
	dirs := lsDirs(filepath.Join(dir, "RPMS"))
	dirs = append(
		dirs,
		filepath.Join(dir, "SRPMS"),
		filepath.Join(dir, "OTHER"),
	)

	r := []job.File{}

	for _, dir := range dirs {
		v := lsFiles(dir)
		for _, name := range v {
			if name != "same_result_marker" && name != ".kiwitree" {
				r = append(r, job.File{
					Name: name,
					Path: filepath.Join(dir, name),
				})
			}
		}
	}

	return r
}
