package container

import (
	"fmt"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/cli"
	dockeropts "github.com/docker/docker/opts"
	"github.com/docker/engine-api/types"
	"github.com/spf13/cobra"
)

type commitOptions struct {
	container string
	reference string

	pause   bool
	comment string
	author  string
	changes dockeropts.ListOpts
}

// NewCommitCommand creates a new cobra.Command for `docker commit`
func NewCommitCommand(dockerCli *client.DockerCli) *cobra.Command {
	var opts commitOptions

	cmd := &cobra.Command{
		Use:   "commit [OPTIONS] CONTAINER [REPOSITORY[:TAG]]",
		Short: "从一个容器的变化部分创建一个新的镜像",
		Args:  cli.RequiresRangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.container = args[0]
			if len(args) > 1 {
				opts.reference = args[1]
			}
			return runCommit(dockerCli, &opts)
		},
	}

	flags := cmd.Flags()
	flags.SetInterspersed(false)

	flags.BoolVarP(&opts.pause, "pause", "p", true, "在容器提交镜像过程中先暂停容器运行")
	flags.StringVarP(&opts.comment, "message", "m", "", "容器提交镜像的消息")
	flags.StringVarP(&opts.author, "author", "a", "", "执行提交操作的作者 (比如, \"张三 <hannibal@a-team.com>\")")

	opts.changes = dockeropts.NewListOpts(nil)
	flags.VarP(&opts.changes, "change", "c", "在创建的镜像中添加Dockerfile中的指令")

	return cmd
}

func runCommit(dockerCli *client.DockerCli, opts *commitOptions) error {
	ctx := context.Background()

	name := opts.container
	reference := opts.reference

	options := types.ContainerCommitOptions{
		Reference: reference,
		Comment:   opts.comment,
		Author:    opts.author,
		Changes:   opts.changes.GetAll(),
		Pause:     opts.pause,
	}

	response, err := dockerCli.Client().ContainerCommit(ctx, name, options)
	if err != nil {
		return err
	}

	fmt.Fprintln(dockerCli.Out(), response.ID)
	return nil
}
