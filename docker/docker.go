package main

import (
	"fmt"
	"os"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/client"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/dockerversion"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/pkg/reexec"
	"github.com/docker/docker/pkg/term"
	"github.com/docker/docker/utils"
)

// 这里是整个docker的main函数，也是执行入口
// 不管是docker daemon还是docker cli
func main() {
	// reexec.Init()的作用，可以参照Docker源码分析
	if reexec.Init() {
		return
	}

	// Set terminal emulation based on platform as required.
	// 获取到stdin,stdout和stderr的文件句柄, 其实就是os.Stdin, os.Stdout, os.Stderr,
	stdin, stdout, stderr := term.StdStreams()

	// 然后将logrus的日志输出设置为stderr
	logrus.SetOutput(stderr)

	// 将所有的flag命令行参数合并
	flag.Merge(flag.CommandLine, clientFlags.FlagSet, commonFlags.FlagSet)

	// 设置该命令行的用途函数
	flag.Usage = func() {
		fmt.Fprint(stdout, "Usage: docker [OPTIONS] COMMAND [arg...]\n"+daemonUsage+"       docker [ --help | -v | --version ]\n\n")
		fmt.Fprint(stdout, "A self-sufficient runtime for containers.\n\nOptions:\n")

		flag.CommandLine.SetOutput(stdout)
		flag.PrintDefaults()

		help := "\nCommands:\n"

		for _, cmd := range dockerCommands {
			help += fmt.Sprintf("    %-10.10s%s\n", cmd.Name, cmd.Description)
		}

		help += "\nRun 'docker COMMAND --help' for more information on a command."
		fmt.Fprintf(stdout, "%s\n", help)
	}

	// 解析flag参数
	flag.Parse()

	// 如果flag参数中*flVersion为true，则显示版本信息
	// 也就是执行了命令 docker --version
	if *flVersion {
		showVersion()
		return
	}

	if *flHelp {
		// if global flag --help is present, regardless of what other options and commands there are,
		// just print the usage.
		flag.Usage()
		return
	}

	// 创建了一个docker cli
	clientCli := client.NewDockerCli(stdin, stdout, stderr, clientFlags)

	// daemonCli的类型为cli.Handler, 为daemon cli配置必需的所有flag参数
	c := cli.New(clientCli, daemonCli)
	// 下面的代码真正运行了命令类型，比如docker info，下面的代码就运行了info
	if err := c.Run(flag.Args()...); err != nil {
		if sterr, ok := err.(cli.StatusError); ok {
			if sterr.Status != "" {
				fmt.Fprintln(stderr, sterr.Status)
				os.Exit(1)
			}
			os.Exit(sterr.StatusCode)
		}
		fmt.Fprintln(stderr, err)
		os.Exit(1)
	}
}

func showVersion() {
	if utils.ExperimentalBuild() {
		fmt.Printf("Docker version %s, build %s, experimental\n", dockerversion.Version, dockerversion.GitCommit)
	} else {
		fmt.Printf("Docker version %s, build %s\n", dockerversion.Version, dockerversion.GitCommit)
	}
}
