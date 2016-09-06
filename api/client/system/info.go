package system

import (
	"fmt"
	"strings"
	"time"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/docker/docker/utils"
	"github.com/docker/docker/utils/templates"
	"github.com/docker/engine-api/types"
	"github.com/docker/engine-api/types/swarm"
	"github.com/docker/go-units"
	"github.com/spf13/cobra"
)

type infoOptions struct {
	format string
}

// NewInfoCommand creates a new cobra.Command for `docker info`
func NewInfoCommand(dockerCli *client.DockerCli) *cobra.Command {
	var opts infoOptions

	cmd := &cobra.Command{
		Use:   "info [OPTIONS]",
		Short: "显示Docker引擎系统级别的信息",
		Args:  cli.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInfo(dockerCli, &opts)
		},
	}

	flags := cmd.Flags()

	flags.StringVarP(&opts.format, "format", "f", "", "基于指定的Go语言模板格式化命令输出内容")

	return cmd
}

func runInfo(dockerCli *client.DockerCli, opts *infoOptions) error {
	ctx := context.Background()
	info, err := dockerCli.Client().Info(ctx)
	if err != nil {
		return err
	}
	if opts.format == "" {
		return prettyPrintInfo(dockerCli, info)
	}
	return formatInfo(dockerCli, info, opts.format)
}

func prettyPrintInfo(dockerCli *client.DockerCli, info types.Info) error {
	fmt.Fprintf(dockerCli.Out(), "容器数: %d\n", info.Containers)
	fmt.Fprintf(dockerCli.Out(), " 运行中: %d\n", info.ContainersRunning)
	fmt.Fprintf(dockerCli.Out(), " 暂停: %d\n", info.ContainersPaused)
	fmt.Fprintf(dockerCli.Out(), " 停止: %d\n", info.ContainersStopped)
	fmt.Fprintf(dockerCli.Out(), "镜像数: %d\n", info.Images)
	ioutils.FprintfIfNotEmpty(dockerCli.Out(), "Docker引擎版本: %s\n", info.ServerVersion)
	ioutils.FprintfIfNotEmpty(dockerCli.Out(), "存储驱动: %s\n", info.Driver)
	if info.DriverStatus != nil {
		for _, pair := range info.DriverStatus {
			fmt.Fprintf(dockerCli.Out(), " %s: %s\n", pair[0], pair[1])

			// print a warning if devicemapper is using a loopback file
			if pair[0] == "Data loop file" {
				fmt.Fprintln(dockerCli.Err(), " 警告: 环回设备loopback严重不建议在生产环境中使用。详见 `--storage-opt dm.thinpooldev` 来指定一个自定义的块存储设备。")
			}
		}

	}
	if info.SystemStatus != nil {
		for _, pair := range info.SystemStatus {
			fmt.Fprintf(dockerCli.Out(), "%s: %s\n", pair[0], pair[1])
		}
	}
	ioutils.FprintfIfNotEmpty(dockerCli.Out(), "日志驱动: %s\n", info.LoggingDriver)
	ioutils.FprintfIfNotEmpty(dockerCli.Out(), "Cgroup驱动: %s\n", info.CgroupDriver)

	fmt.Fprintf(dockerCli.Out(), "插件: \n")
	fmt.Fprintf(dockerCli.Out(), " 存储:")
	fmt.Fprintf(dockerCli.Out(), " %s", strings.Join(info.Plugins.Volume, " "))
	fmt.Fprintf(dockerCli.Out(), "\n")
	fmt.Fprintf(dockerCli.Out(), " 网络:")
	fmt.Fprintf(dockerCli.Out(), " %s", strings.Join(info.Plugins.Network, " "))
	fmt.Fprintf(dockerCli.Out(), "\n")

	if len(info.Plugins.Authorization) != 0 {
		fmt.Fprintf(dockerCli.Out(), " 认证:")
		fmt.Fprintf(dockerCli.Out(), " %s", strings.Join(info.Plugins.Authorization, " "))
		fmt.Fprintf(dockerCli.Out(), "\n")
	}

	fmt.Fprintf(dockerCli.Out(), "Swarm集群: %v\n", info.Swarm.LocalNodeState)
	if info.Swarm.LocalNodeState != swarm.LocalNodeStateInactive {
		fmt.Fprintf(dockerCli.Out(), " 节点ID: %s\n", info.Swarm.NodeID)
		if info.Swarm.Error != "" {
			fmt.Fprintf(dockerCli.Out(), " 错误: %v\n", info.Swarm.Error)
		}
		fmt.Fprintf(dockerCli.Out(), " 是否是管理者: %v\n", info.Swarm.ControlAvailable)
		if info.Swarm.ControlAvailable {
			fmt.Fprintf(dockerCli.Out(), " 集群ID: %s\n", info.Swarm.Cluster.ID)
			fmt.Fprintf(dockerCli.Out(), " 管理者数量: %d\n", info.Swarm.Managers)
			fmt.Fprintf(dockerCli.Out(), " 节点数量: %d\n", info.Swarm.Nodes)
			fmt.Fprintf(dockerCli.Out(), " 编排:\n")
			fmt.Fprintf(dockerCli.Out(), "  任务历史保留数上限: %d\n", info.Swarm.Cluster.Spec.Orchestration.TaskHistoryRetentionLimit)
			fmt.Fprintf(dockerCli.Out(), " Raft:\n")
			fmt.Fprintf(dockerCli.Out(), "  快照间隔: %d\n", info.Swarm.Cluster.Spec.Raft.SnapshotInterval)
			fmt.Fprintf(dockerCli.Out(), "  心跳时钟: %d\n", info.Swarm.Cluster.Spec.Raft.HeartbeatTick)
			fmt.Fprintf(dockerCli.Out(), "  选举时钟: %d\n", info.Swarm.Cluster.Spec.Raft.ElectionTick)
			fmt.Fprintf(dockerCli.Out(), " 分发器:\n")
			fmt.Fprintf(dockerCli.Out(), "  心跳间隔: %s\n", units.HumanDuration(time.Duration(info.Swarm.Cluster.Spec.Dispatcher.HeartbeatPeriod)))
			fmt.Fprintf(dockerCli.Out(), " CA 配置:\n")
			fmt.Fprintf(dockerCli.Out(), "  过期周期: %s\n", units.HumanDuration(info.Swarm.Cluster.Spec.CAConfig.NodeCertExpiry))
			if len(info.Swarm.Cluster.Spec.CAConfig.ExternalCAs) > 0 {
				fmt.Fprintf(dockerCli.Out(), "  外部 CA:\n")
				for _, entry := range info.Swarm.Cluster.Spec.CAConfig.ExternalCAs {
					fmt.Fprintf(dockerCli.Out(), "    %s: %s\n", entry.Protocol, entry.URL)
				}
			}
		}
		fmt.Fprintf(dockerCli.Out(), " 节点地址: %s\n", info.Swarm.NodeAddr)
	}

	if len(info.Runtimes) > 0 {
		fmt.Fprintf(dockerCli.Out(), "运行时:")
		for name := range info.Runtimes {
			fmt.Fprintf(dockerCli.Out(), " %s", name)
		}
		fmt.Fprint(dockerCli.Out(), "\n")
		fmt.Fprintf(dockerCli.Out(), "默认运行时: %s\n", info.DefaultRuntime)
	}

	fmt.Fprintf(dockerCli.Out(), "安全选项:")
	ioutils.FprintfIfNotEmpty(dockerCli.Out(), " %s", strings.Join(info.SecurityOptions, " "))
	fmt.Fprintf(dockerCli.Out(), "\n")

	ioutils.FprintfIfNotEmpty(dockerCli.Out(), "内核版本: %s\n", info.KernelVersion)
	ioutils.FprintfIfNotEmpty(dockerCli.Out(), "操作系统: %s\n", info.OperatingSystem)
	ioutils.FprintfIfNotEmpty(dockerCli.Out(), "操作系统类型: %s\n", info.OSType)
	ioutils.FprintfIfNotEmpty(dockerCli.Out(), "机器架构: %s\n", info.Architecture)
	fmt.Fprintf(dockerCli.Out(), "CPU数量: %d\n", info.NCPU)
	fmt.Fprintf(dockerCli.Out(), "内存总数: %s\n", units.BytesSize(float64(info.MemTotal)))
	ioutils.FprintfIfNotEmpty(dockerCli.Out(), "名称: %s\n", info.Name)
	ioutils.FprintfIfNotEmpty(dockerCli.Out(), "ID: %s\n", info.ID)
	fmt.Fprintf(dockerCli.Out(), "Docker引擎根目录地址: %s\n", info.DockerRootDir)
	fmt.Fprintf(dockerCli.Out(), "调试模式(客户端): %v\n", utils.IsDebugEnabled())
	fmt.Fprintf(dockerCli.Out(), "调试模式(服务端): %v\n", info.Debug)

	if info.Debug {
		fmt.Fprintf(dockerCli.Out(), " 文件描述符个数: %d\n", info.NFd)
		fmt.Fprintf(dockerCli.Out(), " Go协程个数: %d\n", info.NGoroutines)
		fmt.Fprintf(dockerCli.Out(), " 系统时间: %s\n", info.SystemTime)
		fmt.Fprintf(dockerCli.Out(), " 时间监听者数量: %d\n", info.NEventsListener)
	}

	ioutils.FprintfIfNotEmpty(dockerCli.Out(), "Http代理: %s\n", info.HTTPProxy)
	ioutils.FprintfIfNotEmpty(dockerCli.Out(), "Https代理: %s\n", info.HTTPSProxy)
	ioutils.FprintfIfNotEmpty(dockerCli.Out(), "没有使用代理: %s\n", info.NoProxy)

	if info.IndexServerAddress != "" {
		u := dockerCli.ConfigFile().AuthConfigs[info.IndexServerAddress].Username
		if len(u) > 0 {
			fmt.Fprintf(dockerCli.Out(), "用户名: %v\n", u)
		}
		fmt.Fprintf(dockerCli.Out(), "镜像仓库: %v\n", info.IndexServerAddress)
	}

	// Only output these warnings if the server does not support these features
	if info.OSType != "windows" {
		if !info.MemoryLimit {
			fmt.Fprintln(dockerCli.Err(), "警告: 不支持内存限制")
		}
		if !info.SwapLimit {
			fmt.Fprintln(dockerCli.Err(), "警告: 不支持交换区内存限制")
		}
		if !info.KernelMemory {
			fmt.Fprintln(dockerCli.Err(), "警告: 不支持内核内存限制")
		}
		if !info.OomKillDisable {
			fmt.Fprintln(dockerCli.Err(), "警告: 不支持oom kill 禁用")
		}
		if !info.CPUCfsQuota {
			fmt.Fprintln(dockerCli.Err(), "警告: 不支持 cpu cfs 限额 ")
		}
		if !info.CPUCfsPeriod {
			fmt.Fprintln(dockerCli.Err(), "警告: 不支持 cpu cfs 周期 ")
		}
		if !info.CPUShares {
			fmt.Fprintln(dockerCli.Err(), "警告: 不支持 cpu 时间")
		}
		if !info.CPUSet {
			fmt.Fprintln(dockerCli.Err(), "警告: 不支持 cpuset")
		}
		if !info.IPv4Forwarding {
			fmt.Fprintln(dockerCli.Err(), "警告: IPv4转发功能已禁用")
		}
		if !info.BridgeNfIptables {
			fmt.Fprintln(dockerCli.Err(), "警告: bridge-nf-call-iptables已禁用")
		}
		if !info.BridgeNfIP6tables {
			fmt.Fprintln(dockerCli.Err(), "警告: bridge-nf-call-ip6tables已禁用")
		}
	}

	if info.Labels != nil {
		fmt.Fprintln(dockerCli.Out(), "标签:")
		for _, attribute := range info.Labels {
			fmt.Fprintf(dockerCli.Out(), " %s\n", attribute)
		}
	}

	ioutils.FprintfIfTrue(dockerCli.Out(), "试验版本: %v\n", info.ExperimentalBuild)
	if info.ClusterStore != "" {
		fmt.Fprintf(dockerCli.Out(), "集群存储: %s\n", info.ClusterStore)
	}

	if info.ClusterAdvertise != "" {
		fmt.Fprintf(dockerCli.Out(), "集群广播地址: %s\n", info.ClusterAdvertise)
	}

	if info.RegistryConfig != nil && (len(info.RegistryConfig.InsecureRegistryCIDRs) > 0 || len(info.RegistryConfig.IndexConfigs) > 0) {
		fmt.Fprintln(dockerCli.Out(), "不受信的镜像仓库:")
		for _, registry := range info.RegistryConfig.IndexConfigs {
			if registry.Secure == false {
				fmt.Fprintf(dockerCli.Out(), " %s\n", registry.Name)
			}
		}

		for _, registry := range info.RegistryConfig.InsecureRegistryCIDRs {
			mask, _ := registry.Mask.Size()
			fmt.Fprintf(dockerCli.Out(), " %s/%d\n", registry.IP.String(), mask)
		}
	}

	fmt.Fprintf(dockerCli.Out(), "启用实时容器加载: %v\n", info.LiveRestoreEnabled)

	return nil
}

func formatInfo(dockerCli *client.DockerCli, info types.Info, format string) error {
	tmpl, err := templates.Parse(format)
	if err != nil {
		return cli.StatusError{StatusCode: 64,
			Status: "Template parsing error: " + err.Error()}
	}
	err = tmpl.Execute(dockerCli.Out(), info)
	dockerCli.Out().Write([]byte{'\n'})
	return err
}
