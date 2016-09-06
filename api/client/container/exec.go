package container

import (
	"fmt"
	"io"

	"golang.org/x/net/context"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/client"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/pkg/promise"
	"github.com/docker/engine-api/types"
	"github.com/spf13/cobra"
)

type execOptions struct {
	detachKeys  string
	interactive bool
	tty         bool
	detach      bool
	user        string
	privileged  bool
}

// NewExecCommand creats a new cobra.Command for `docker exec`
func NewExecCommand(dockerCli *client.DockerCli) *cobra.Command {
	var opts execOptions

	cmd := &cobra.Command{
		Use:   "exec [OPTIONS] CONTAINER COMMAND [ARG...]",
		Short: "在一个运行的容器运行用户指定的命令，常用来调试",
		Args:  cli.RequiresMinArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			container := args[0]
			execCmd := args[1:]
			return runExec(dockerCli, &opts, container, execCmd)
		},
	}

	flags := cmd.Flags()
	flags.SetInterspersed(false)

	flags.StringVarP(&opts.detachKeys, "detach-keys", "", "", "覆盖从一个容器停止附加时的退出按键")
	flags.BoolVarP(&opts.interactive, "interactive", "i", false, "即使容器没有被附加标准输出，标准错误，也爆出输出输入的畅通")
	flags.BoolVarP(&opts.tty, "tty", "t", false, "分配一个伪终端")
	flags.BoolVarP(&opts.detach, "detach", "d", false, "后台模式: 在后台运行用户指定的命令")
	flags.StringVarP(&opts.user, "user", "u", "", "用户名或用户名ID (格式: <用户名|用户名ID>[:<组|组ID>])")
	flags.BoolVarP(&opts.privileged, "privileged", "", false, "为运行命令授予格外的特权")

	return cmd
}

func runExec(dockerCli *client.DockerCli, opts *execOptions, container string, execCmd []string) error {
	execConfig, err := parseExec(opts, container, execCmd)
	// just in case the ParseExec does not exit
	if container == "" || err != nil {
		return cli.StatusError{StatusCode: 1}
	}

	if opts.detachKeys != "" {
		dockerCli.ConfigFile().DetachKeys = opts.detachKeys
	}

	// Send client escape keys
	execConfig.DetachKeys = dockerCli.ConfigFile().DetachKeys

	ctx := context.Background()

	response, err := dockerCli.Client().ContainerExecCreate(ctx, container, *execConfig)
	if err != nil {
		return err
	}

	execID := response.ID
	if execID == "" {
		fmt.Fprintf(dockerCli.Out(), "执行ID为空")
		return nil
	}

	//Temp struct for execStart so that we don't need to transfer all the execConfig
	if !execConfig.Detach {
		if err := dockerCli.CheckTtyInput(execConfig.AttachStdin, execConfig.Tty); err != nil {
			return err
		}
	} else {
		execStartCheck := types.ExecStartCheck{
			Detach: execConfig.Detach,
			Tty:    execConfig.Tty,
		}

		if err := dockerCli.Client().ContainerExecStart(ctx, execID, execStartCheck); err != nil {
			return err
		}
		// For now don't print this - wait for when we support exec wait()
		// fmt.Fprintf(dockerCli.Out(), "%s\n", execID)
		return nil
	}

	// Interactive exec requested.
	var (
		out, stderr io.Writer
		in          io.ReadCloser
		errCh       chan error
	)

	if execConfig.AttachStdin {
		in = dockerCli.In()
	}
	if execConfig.AttachStdout {
		out = dockerCli.Out()
	}
	if execConfig.AttachStderr {
		if execConfig.Tty {
			stderr = dockerCli.Out()
		} else {
			stderr = dockerCli.Err()
		}
	}

	resp, err := dockerCli.Client().ContainerExecAttach(ctx, execID, *execConfig)
	if err != nil {
		return err
	}
	defer resp.Close()
	errCh = promise.Go(func() error {
		return dockerCli.HoldHijackedConnection(ctx, execConfig.Tty, in, out, stderr, resp)
	})

	if execConfig.Tty && dockerCli.IsTerminalIn() {
		if err := dockerCli.MonitorTtySize(ctx, execID, true); err != nil {
			fmt.Fprintf(dockerCli.Err(), "监听TTY规格出错: %s\n", err)
		}
	}

	if err := <-errCh; err != nil {
		logrus.Debugf("Error hijack: %s", err)
		return err
	}

	var status int
	if _, status, err = dockerCli.GetExecExitCode(ctx, execID); err != nil {
		return err
	}

	if status != 0 {
		return cli.StatusError{StatusCode: status}
	}

	return nil
}

// parseExec parses the specified args for the specified command and generates
// an ExecConfig from it.
func parseExec(opts *execOptions, container string, execCmd []string) (*types.ExecConfig, error) {
	execConfig := &types.ExecConfig{
		User:       opts.user,
		Privileged: opts.privileged,
		Tty:        opts.tty,
		Cmd:        execCmd,
		Detach:     opts.detach,
		// container is not used here
	}

	// If -d is not set, attach to everything by default
	if !opts.detach {
		execConfig.AttachStdout = true
		execConfig.AttachStderr = true
		if opts.interactive {
			execConfig.AttachStdin = true
		}
	}

	return execConfig, nil
}
