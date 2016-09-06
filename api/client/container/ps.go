package container

import (
	"golang.org/x/net/context"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/api/client/formatter"
	"github.com/docker/docker/cli"
	"github.com/docker/engine-api/types"
	"github.com/docker/engine-api/types/filters"

	"io/ioutil"

	"github.com/docker/docker/utils/templates"
	"github.com/spf13/cobra"
)

type psOptions struct {
	quiet   bool
	size    bool
	all     bool
	noTrunc bool
	nLatest bool
	last    int
	format  string
	filter  []string
}

// NewPsCommand creates a new cobra.Command for `docker ps`
func NewPsCommand(dockerCli *client.DockerCli) *cobra.Command {
	var opts psOptions

	cmd := &cobra.Command{
		Use:   "ps [OPTIONS]",
		Short: "罗列所有容器",
		Args:  cli.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPs(dockerCli, &opts)
		},
	}

	flags := cmd.Flags()

	flags.BoolVarP(&opts.quiet, "quiet", "q", false, "仅显示容器ID")
	flags.BoolVarP(&opts.size, "size", "s", false, "显示所有的文件大小")
	flags.BoolVarP(&opts.all, "all", "a", false, "显示所有的容器(默认仅显示运行的容器)")
	flags.BoolVar(&opts.noTrunc, "no-trunc", false, "不截断输出")
	flags.BoolVarP(&opts.nLatest, "latest", "l", false, "显示最新创建的容器(包含所有的状态)")
	flags.IntVarP(&opts.last, "last", "n", -1, "显示n个最新创建的容器(包含所有的状态)")
	flags.StringVarP(&opts.format, "format", "", "", "使用一个Go语言的模板打印容器")
	flags.StringSliceVarP(&opts.filter, "filter", "f", []string{}, "基于指定的条件过滤命令输出内容")

	return cmd
}

type preProcessor struct {
	types.Container
	opts *types.ContainerListOptions
}

// Size sets the size option when called by a template execution.
func (p *preProcessor) Size() bool {
	p.opts.Size = true
	return true
}

func buildContainerListOptions(opts *psOptions) (*types.ContainerListOptions, error) {

	options := &types.ContainerListOptions{
		All:    opts.all,
		Limit:  opts.last,
		Size:   opts.size,
		Filter: filters.NewArgs(),
	}

	if opts.nLatest && opts.last == -1 {
		options.Limit = 1
	}

	for _, f := range opts.filter {
		var err error
		options.Filter, err = filters.ParseFlag(f, options.Filter)
		if err != nil {
			return nil, err
		}
	}

	// Currently only used with Size, so we can determine if the user
	// put {{.Size}} in their format.
	pre := &preProcessor{opts: options}
	tmpl, err := templates.Parse(opts.format)

	if err != nil {
		return nil, err
	}

	// This shouldn't error out but swallowing the error makes it harder
	// to track down if preProcessor issues come up. Ref #24696
	if err := tmpl.Execute(ioutil.Discard, pre); err != nil {
		return nil, err
	}

	return options, nil
}

func runPs(dockerCli *client.DockerCli, opts *psOptions) error {
	ctx := context.Background()

	listOptions, err := buildContainerListOptions(opts)
	if err != nil {
		return err
	}

	containers, err := dockerCli.Client().ContainerList(ctx, *listOptions)
	if err != nil {
		return err
	}

	f := opts.format
	if len(f) == 0 {
		if len(dockerCli.ConfigFile().PsFormat) > 0 && !opts.quiet {
			f = dockerCli.ConfigFile().PsFormat
		} else {
			f = "table"
		}
	}

	psCtx := formatter.ContainerContext{
		Context: formatter.Context{
			Output: dockerCli.Out(),
			Format: f,
			Quiet:  opts.quiet,
			Trunc:  !opts.noTrunc,
		},
		Size:       listOptions.Size,
		Containers: containers,
	}

	psCtx.Write()

	return nil
}
