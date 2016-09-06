package container

import (
	"fmt"
	"strings"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/cli"
	runconfigopts "github.com/docker/docker/runconfig/opts"
	containertypes "github.com/docker/engine-api/types/container"
	"github.com/docker/go-units"
	"github.com/spf13/cobra"
)

type updateOptions struct {
	blkioWeight       uint16
	cpuPeriod         int64
	cpuQuota          int64
	cpusetCpus        string
	cpusetMems        string
	cpuShares         int64
	memoryString      string
	memoryReservation string
	memorySwap        string
	kernelMemory      string
	restartPolicy     string

	nFlag int

	containers []string
}

// NewUpdateCommand creates a new cobra.Command for `docker update`
func NewUpdateCommand(dockerCli *client.DockerCli) *cobra.Command {
	var opts updateOptions

	cmd := &cobra.Command{
		Use:   "update [OPTIONS] CONTAINER [CONTAINER...]",
		Short: "更新一个或多个容器的配置信息",
		Args:  cli.RequiresMinArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.containers = args
			opts.nFlag = cmd.Flags().NFlag()
			return runUpdate(dockerCli, &opts)
		},
	}

	flags := cmd.Flags()
	flags.Uint16Var(&opts.blkioWeight, "blkio-weight", 0, "磁盘IO设置(相对值),从10到1000")
	flags.Int64Var(&opts.cpuPeriod, "cpu-period", 0, "限制CPU绝对公平调度算法（CFS）的时间周期")
	flags.Int64Var(&opts.cpuQuota, "cpu-quota", 0, "限制CPU绝对公平调度算法（CFS）的时间限额")
	flags.StringVar(&opts.cpusetCpus, "cpuset-cpus", "", "允许容器执行的CPU核指定(0-3,0,1): 0-3代表运行运行在0,1,2,3这4个核上")
	flags.StringVar(&opts.cpusetMems, "cpuset-mems", "", "允许容器执行的CPU内存所在核指定(0-3,0,1): 0-3代表运行运行在0,1,2,3这4个核上")
	flags.Int64VarP(&opts.cpuShares, "cpu-shares", "c", 0, "CPU计算资源的值(相对值)")
	flags.StringVarP(&opts.memoryString, "memory", "m", "", "内存限制")
	flags.StringVar(&opts.memoryReservation, "memory-reservation", "", "内存软限制")
	flags.StringVar(&opts.memorySwap, "memory-swap", "", "交换内存限制 等于 实际内存 ＋ 交换区内存: '-1' 代表启用不受限的交换区内存")
	flags.StringVar(&opts.kernelMemory, "kernel-memory", "", "内核内存限制")
	flags.StringVar(&opts.restartPolicy, "restart", "", "当容器退出时应用在容器上的重启策略")

	return cmd
}

func runUpdate(dockerCli *client.DockerCli, opts *updateOptions) error {
	var err error

	if opts.nFlag == 0 {
		return fmt.Errorf("当使用此命令时，您必须提供一个或多个命令参数。")
	}

	var memory int64
	if opts.memoryString != "" {
		memory, err = units.RAMInBytes(opts.memoryString)
		if err != nil {
			return err
		}
	}

	var memoryReservation int64
	if opts.memoryReservation != "" {
		memoryReservation, err = units.RAMInBytes(opts.memoryReservation)
		if err != nil {
			return err
		}
	}

	var memorySwap int64
	if opts.memorySwap != "" {
		if opts.memorySwap == "-1" {
			memorySwap = -1
		} else {
			memorySwap, err = units.RAMInBytes(opts.memorySwap)
			if err != nil {
				return err
			}
		}
	}

	var kernelMemory int64
	if opts.kernelMemory != "" {
		kernelMemory, err = units.RAMInBytes(opts.kernelMemory)
		if err != nil {
			return err
		}
	}

	var restartPolicy containertypes.RestartPolicy
	if opts.restartPolicy != "" {
		restartPolicy, err = runconfigopts.ParseRestartPolicy(opts.restartPolicy)
		if err != nil {
			return err
		}
	}

	resources := containertypes.Resources{
		BlkioWeight:       opts.blkioWeight,
		CpusetCpus:        opts.cpusetCpus,
		CpusetMems:        opts.cpusetMems,
		CPUShares:         opts.cpuShares,
		Memory:            memory,
		MemoryReservation: memoryReservation,
		MemorySwap:        memorySwap,
		KernelMemory:      kernelMemory,
		CPUPeriod:         opts.cpuPeriod,
		CPUQuota:          opts.cpuQuota,
	}

	updateConfig := containertypes.UpdateConfig{
		Resources:     resources,
		RestartPolicy: restartPolicy,
	}

	ctx := context.Background()

	var (
		warns []string
		errs  []string
	)
	for _, container := range opts.containers {
		r, err := dockerCli.Client().ContainerUpdate(ctx, container, updateConfig)
		if err != nil {
			errs = append(errs, err.Error())
		} else {
			fmt.Fprintf(dockerCli.Out(), "%s\n", container)
		}
		warns = append(warns, r.Warnings...)
	}
	if len(warns) > 0 {
		fmt.Fprintf(dockerCli.Out(), "%s", strings.Join(warns, "\n"))
	}
	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "\n"))
	}
	return nil
}
