package swarm

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/cli"
	"github.com/docker/engine-api/types/swarm"
	"golang.org/x/net/context"
)

func newJoinTokenCommand(dockerCli *client.DockerCli) *cobra.Command {
	var rotate, quiet bool

	cmd := &cobra.Command{
		Use:   "join-token [OPTIONS] (worker|manager)",
		Short: "管理加群令牌",
		Args:  cli.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			worker := args[0] == "worker"
			manager := args[0] == "manager"

			if !worker && !manager {
				return errors.New("未知的角色 " + args[0])
			}

			client := dockerCli.Client()
			ctx := context.Background()

			if rotate {
				var flags swarm.UpdateFlags

				swarm, err := client.SwarmInspect(ctx)
				if err != nil {
					return err
				}

				flags.RotateWorkerToken = worker
				flags.RotateManagerToken = manager

				err = client.SwarmUpdate(ctx, swarm.Version, swarm.Spec, flags)
				if err != nil {
					return err
				}
				if !quiet {
					fmt.Fprintf(dockerCli.Out(), "成功轮换 %s 加入令牌。\n\n", args[0])
				}
			}

			swarm, err := client.SwarmInspect(ctx)
			if err != nil {
				return err
			}

			if quiet {
				if worker {
					fmt.Fprintln(dockerCli.Out(), swarm.JoinTokens.Worker)
				} else {
					fmt.Fprintln(dockerCli.Out(), swarm.JoinTokens.Manager)
				}
			} else {
				info, err := client.Info(ctx)
				if err != nil {
					return err
				}
				return printJoinCommand(ctx, dockerCli, info.Swarm.NodeID, worker, manager)
			}
			return nil
		},
	}

	flags := cmd.Flags()
	flags.BoolVar(&rotate, flagRotate, false, "轮换加入令牌")
	flags.BoolVarP(&quiet, flagQuiet, "q", false, "仅显示令牌")

	return cmd
}

func printJoinCommand(ctx context.Context, dockerCli *client.DockerCli, nodeID string, worker bool, manager bool) error {
	client := dockerCli.Client()

	swarm, err := client.SwarmInspect(ctx)
	if err != nil {
		return err
	}

	node, _, err := client.NodeInspectWithRaw(ctx, nodeID)
	if err != nil {
		return err
	}

	if node.ManagerStatus != nil {
		if worker {
			fmt.Fprintf(dockerCli.Out(), "如需在此Swarm集群中加入一个工作者，在工作者节点上运行以下命令:\n\n    docker swarm join \\\n    --token %s \\\n    %s\n\n", swarm.JoinTokens.Worker, node.ManagerStatus.Addr)
		}
		if manager {
			fmt.Fprintf(dockerCli.Out(), "如需在此Swarm集群中加入一个管理者，在管理者节点上运行以下命令:\n\n    docker swarm join \\\n    --token %s \\\n    %s\n\n", swarm.JoinTokens.Manager, node.ManagerStatus.Addr)
		}
	}

	return nil
}
