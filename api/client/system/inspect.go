package system

import (
	"fmt"
	"strings"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/api/client/inspect"
	"github.com/docker/docker/cli"
	apiclient "github.com/docker/engine-api/client"
	"github.com/spf13/cobra"
)

type inspectOptions struct {
	format      string
	inspectType string
	size        bool
	ids         []string
}

// NewInspectCommand creates a new cobra.Command for `docker inspect`
func NewInspectCommand(dockerCli *client.DockerCli) *cobra.Command {
	var opts inspectOptions

	cmd := &cobra.Command{
		Use:   "inspect [OPTIONS] CONTAINER|IMAGE|TASK [CONTAINER|IMAGE|TASK...]",
		Short: "返回一个容器，或镜像，或任务的底层具体信息",
		Args:  cli.RequiresMinArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.ids = args
			return runInspect(dockerCli, opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&opts.format, "format", "f", "", "基于指定的Go语言模板格式化命令输出内容")
	flags.StringVar(&opts.inspectType, "type", "", "为指定的类型返回JSON内容")
	flags.BoolVarP(&opts.size, "size", "s", false, "如果类型为容器，显示所有的文件大小信息")

	return cmd
}

func runInspect(dockerCli *client.DockerCli, opts inspectOptions) error {
	var elementSearcher inspect.GetRefFunc
	switch opts.inspectType {
	case "", "container", "image", "node", "network", "service", "volume", "task":
		elementSearcher = inspectAll(context.Background(), dockerCli, opts.size, opts.inspectType)
	default:
		return fmt.Errorf("对 --type而言，%q 不是一个有效的值", opts.inspectType)
	}
	return inspect.Inspect(dockerCli.Out(), opts.ids, opts.format, elementSearcher)
}

func inspectContainers(ctx context.Context, dockerCli *client.DockerCli, getSize bool) inspect.GetRefFunc {
	return func(ref string) (interface{}, []byte, error) {
		return dockerCli.Client().ContainerInspectWithRaw(ctx, ref, getSize)
	}
}

func inspectImages(ctx context.Context, dockerCli *client.DockerCli) inspect.GetRefFunc {
	return func(ref string) (interface{}, []byte, error) {
		return dockerCli.Client().ImageInspectWithRaw(ctx, ref)
	}
}

func inspectNetwork(ctx context.Context, dockerCli *client.DockerCli) inspect.GetRefFunc {
	return func(ref string) (interface{}, []byte, error) {
		return dockerCli.Client().NetworkInspectWithRaw(ctx, ref)
	}
}

func inspectNode(ctx context.Context, dockerCli *client.DockerCli) inspect.GetRefFunc {
	return func(ref string) (interface{}, []byte, error) {
		return dockerCli.Client().NodeInspectWithRaw(ctx, ref)
	}
}

func inspectService(ctx context.Context, dockerCli *client.DockerCli) inspect.GetRefFunc {
	return func(ref string) (interface{}, []byte, error) {
		return dockerCli.Client().ServiceInspectWithRaw(ctx, ref)
	}
}

func inspectTasks(ctx context.Context, dockerCli *client.DockerCli) inspect.GetRefFunc {
	return func(ref string) (interface{}, []byte, error) {
		return dockerCli.Client().TaskInspectWithRaw(ctx, ref)
	}
}

func inspectVolume(ctx context.Context, dockerCli *client.DockerCli) inspect.GetRefFunc {
	return func(ref string) (interface{}, []byte, error) {
		return dockerCli.Client().VolumeInspectWithRaw(ctx, ref)
	}
}

func inspectAll(ctx context.Context, dockerCli *client.DockerCli, getSize bool, typeConstraint string) inspect.GetRefFunc {
	var inspectAutodetect = []struct {
		ObjectType      string
		IsSizeSupported bool
		ObjectInspector func(string) (interface{}, []byte, error)
	}{
		{"container", true, inspectContainers(ctx, dockerCli, getSize)},
		{"image", true, inspectImages(ctx, dockerCli)},
		{"network", false, inspectNetwork(ctx, dockerCli)},
		{"volume", false, inspectVolume(ctx, dockerCli)},
		{"service", false, inspectService(ctx, dockerCli)},
		{"task", false, inspectTasks(ctx, dockerCli)},
		{"node", false, inspectNode(ctx, dockerCli)},
	}

	isErrNotSwarmManager := func(err error) bool {
		return strings.Contains(err.Error(), "This node is not a swarm manager")
	}

	return func(ref string) (interface{}, []byte, error) {
		for _, inspectData := range inspectAutodetect {
			if typeConstraint != "" && inspectData.ObjectType != typeConstraint {
				continue
			}
			v, raw, err := inspectData.ObjectInspector(ref)
			if err != nil {
				if typeConstraint == "" && (apiclient.IsErrNotFound(err) || isErrNotSwarmManager(err)) {
					continue
				}
				return v, raw, err
			}
			if !inspectData.IsSizeSupported {
				fmt.Fprintf(dockerCli.Err(), "警告: --size 被忽略 %s\n", inspectData.ObjectType)
			}
			return v, raw, err
		}
		return nil, nil, fmt.Errorf("错误: 没有该项目: %s", ref)
	}
}
