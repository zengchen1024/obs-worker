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

	stage          string
	needOBSPackage bool
}

func newNonModeBuild(workDir string, cfg *Config, info *buildinfo.BuildInfo) (*nonModeBuid, error) {
	b := nonModeBuid{
		buildHelper: buildHelper{
			cfg:     cfg,
			info:    BuildInfo{BuildInfo: *info},
			workDir: workDir,
		},
		stage: BuildStagePrepare,
	}

	h := &b.buildHelper
	h.init()

	b.sources.buildHelper = h
	b.rpmlist.buildHelper = h
	b.build.buildHelper = h
	b.cache.init(h)

	b.oldpkg.buildHelper = h
	b.oldpkg.handleDownloadDetails = b.stats.setBinaryDownloadDetail

	bm := &b.binaryManager
	bm.buildHelper = h
	bm.cache = &b.cache
	bm.handleCacheHits = b.stats.setCacheHit
	bm.handleDownloadDetails = b.stats.setBinaryDownloadDetail
	bm.init()

	im := &b.imageManager
	im.buildHelper = h
	im.cache = &b.cache
	im.handleRepoBins = b.binaryManager.setKnownBins

	bl := &b.binaryLoader
	bl.buildHelper = h
	bl.imageManager = &b.imageManager
	bl.handleOutBDep = b.out.setBdep
	bl.binaryManager = &b.binaryManager
	bl.handleKiwiOrigin = b.report.setKiwiOrigin
	bl.handleImage = func(img *preInstallImage) {
		b.rpmlist.setPreInstallImage(img)
		b.stats.setPreInstallImage(img)
	}

	return &b, nil
}

func (b *nonModeBuid) preBuild() error {
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

	utils.LogInfo("start downloading project config")

	if err := b.downloadProjectConfig(); err != nil {
		return err
	}

	utils.LogInfo("start generating rpmlist")

	if err := b.rpmlist.generate(); err != nil {
		return err
	}

	return nil
}

func (b *nonModeBuid) DoBuild(jobId string) (int, error) {
	if err := b.preBuild(); err != nil {
		return 0, err
	}

	utils.LogInfo("start building")

	b.stage = BuildStageBuilding

	if c, err := b.build.do(); err != nil {
		return c, err
	}

	utils.LogInfo("start post build")

	b.stage = BuildStagePostBuild

	dir := b.env.otherDir

	mkdirAll(dir)

	b.stats.do(dir)

	b.out.writeBuildEnv(dir)

	b.report.do(dir)

	b.postBuild(jobId)

	return 0, nil
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
	utils.LogInfo("start getting sources")

	s, err := b.sources.getSource()
	if err != nil {
		return err
	}

	metas := []string{s}

	needSSLCert, ignoreImage, _ := b.parseBuildFile()

	if needSSLCert {
		utils.LogInfo("start downloading sslcert")

		if err := b.sources.downloadSSLCert(); err != nil {
			return err
		}
	}

	if ignoreImage {
		b.report.collectOrigins = true
	}

	utils.LogInfo("start getting binaries")

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

func (b *nonModeBuid) Kill() error {
	b.setCancel()
	return b.build.kill()
}

func (b *nonModeBuid) GetBuildStage() string {
	return b.stage
}

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

	err := job.Put(info.RepoServer, opt, files)
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
