// +build experimental

package plugin

import (
	"fmt"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/api/client/inspect"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/reference"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
)

type inspectOptions struct {
	pluginNames []string
	format      string
}

func newInspectCommand(dockerCli *client.DockerCli) *cobra.Command {
	var opts inspectOptions

	cmd := &cobra.Command{
		Use:   "inspect [OPTIONS] PLUGIN [PLUGIN...]",
		Short: "显示一个或多个插件的详细信息",
		Args:  cli.RequiresMinArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.pluginNames = args
			return runInspect(dockerCli, opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&opts.format, "format", "f", "", "通过指定的Go语言模板格式化命令输出内容")
	return cmd
}

func runInspect(dockerCli *client.DockerCli, opts inspectOptions) error {
	client := dockerCli.Client()
	ctx := context.Background()
	getRef := func(name string) (interface{}, []byte, error) {
		named, err := reference.ParseNamed(name) // FIXME: validate
		if err != nil {
			return nil, nil, err
		}
		if reference.IsNameOnly(named) {
			named = reference.WithDefaultTag(named)
		}
		ref, ok := named.(reference.NamedTagged)
		if !ok {
			return nil, nil, fmt.Errorf("invalid name: %s", named.String())
		}

		return client.PluginInspectWithRaw(ctx, ref.String())
	}

	return inspect.Inspect(dockerCli.Out(), opts.pluginNames, opts.format, getRef)
}
