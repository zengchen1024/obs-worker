package worker

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/zengchen1024/obs-worker/sdk/worker"
	"github.com/zengchen1024/obs-worker/utils"
)

func (b *buildManager) sendIdleState() {
	state := "idle"

	opts := worker.QueryOpts{
		WorkerId: b.cfg.Id,
		State:    state,
		Port:     b.port,
		Arch:     b.cfg.HostArch,
	}

	for _, server := range b.cfg.RepoServers {
		b.w.RegisterServer = server

		utils.LogInfo("register to %s", server)

		if err := worker.Create(&b.hc, server, &opts, &b.w); err != nil {
			utils.LogErr("send idle state, err:%v", err)
		}
	}
}

func (b *buildManager) sendExitState() {
	state := "exit"

	opts := worker.QueryOpts{
		WorkerId: b.cfg.Id,
		State:    state,
		Port:     b.port,
		Arch:     b.cfg.HostArch,
	}

	for _, server := range b.cfg.RepoServers {
		if err := worker.Get(&b.hc, server, &opts); err != nil {
			utils.LogErr("send exit state, err:%v", err)
		}
	}
}

func (b *buildManager) getWorkerInfo() error {
	cfg := b.cfg

	info := &b.w
	info.HostLabel = cfg.HostLabel
	info.Owner = cfg.Owner

	if t := cfg.GetVMType(); t != "" {
		info.Sandbox = t
	} else {
		info.Sandbox = "chroot"
	}

	if err := b.getLinuxVersion(info); err != nil {
		return err
	}

	b.getWorkerCPUInfo(info)

	b.getWorkerHardware(info)

	return nil
}

func (b *buildManager) getLinuxVersion(w *worker.Worker) error {
	bs, err := os.ReadFile("/proc/version")
	if err != nil {
		return err
	}

	re := regexp.MustCompile("^Linux version ([^ ]*)-([^- ]*) ")
	if re.Match(bs) {
		v := re.FindStringSubmatch(string(bs))
		w.Linux = worker.Linux{
			Version: v[1],
			Flavor:  v[2],
		}
	}

	return nil
}

func (b *buildManager) getWorkerCPUInfo(w *worker.Worker) {
	implementer := ""
	variant := ""
	processor := 0
	var flags []string

	re := regexp.MustCompile("[a-zA-Z ]*\\s*:\\s(.*)\\s*$")

	err := utils.ReadFileLineByLine("/proc/cpuinfo", func(l string) (b bool) {
		if strings.HasPrefix(l, "processors") {
			processor += 1
			return
		}

		if strings.HasPrefix(l, "flags") || strings.HasPrefix(l, "Features") {
			if v := re.FindStringSubmatch(l); v != nil {
				flags = strings.Split(v[1], " ")
			}

			return
		}

		if strings.HasPrefix(l, "CPU implementer") {
			if v := re.FindStringSubmatch(l); v != nil {
				implementer = v[1]
			}

			return
		}

		if strings.HasPrefix(l, "CPU variant") {
			if v := re.FindStringSubmatch(l); v != nil {
				variant = v[1]
			}

			return
		}

		if strings.HasPrefix(l, "cpu") {
			if v := re.FindStringSubmatch(l); v != nil {
				switch v[1] {
				case "POWER9":
					flags = []string{"power7", "power8", "power9"}
				case "POWER8":
					flags = []string{"power7", "power8"}
				case "POWER7":
					flags = []string{"power7"}
				}
			}
		}

		return
	})

	if err != nil {
		return
	}

	w.Hardware.CPU.Flag = flags
	w.Hardware.Processors = processor

	if b.cfg.HostArch == "aarch64" && implementer != "" {
		aarch32 := false

		switch implementer {
		case "0x50":
			aarch32 = variant == "0x0" || variant == "0x1"

		case "0x41":
			aarch32 = true
		}

		if aarch32 {
			w.Hardware.NativeOnly = true
		}
	}
}

func (b *buildManager) getWorkerHardware(w *worker.Worker) {
	vm := b.cfg.GetVMInfo()
	if vm == nil {
		return
	}

	hw := &w.Hardware

	hw.Jobs = b.cfg.Jobs
	hw.Memory = vm.Memory
	hw.Swap = vm.SwapSize
	hw.Disk = vm.RootSize

	if b.cfg.IsVMOpenstack() {
		hw.Processors = b.cfg.Jobs
		return
	}

	if vm.Device != "" {
		if v, err := getDeviceSize(vm.Device); err == nil {
			hw.Disk = v
		}
	}

	if vm.Swap != "" {
		if v, err := getDeviceSize(vm.Swap); err == nil {
			hw.Swap = v
		}
	}
}

func getDeviceSize(device string) (int, error) {
	//TODO
	return 0, fmt.Errorf("unimplemented")
}
