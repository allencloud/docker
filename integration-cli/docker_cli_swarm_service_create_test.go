// +build !windows

package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/integration-cli/checker"
	"github.com/docker/docker/integration-cli/cli"
	"github.com/docker/docker/pkg/testutil"
	icmd "github.com/docker/docker/pkg/testutil/cmd"
	"github.com/go-check/check"
)

func (s *DockerSwarmSuite) TestServiceCreateMountVolume(c *check.C) {
	d := s.AddDaemon(c, true, true)
	out, err := d.Cmd("service", "create", "--no-resolve-image", "--detach=true", "--mount", "type=volume,source=foo,target=/foo,volume-nocopy", "busybox", "top")
	c.Assert(err, checker.IsNil, check.Commentf(out))
	id := strings.TrimSpace(out)

	var tasks []swarm.Task
	waitAndAssert(c, defaultReconciliationTimeout, func(c *check.C) (interface{}, check.CommentInterface) {
		tasks = d.GetServiceTasks(c, id)
		return len(tasks) > 0, nil
	}, checker.Equals, true)

	task := tasks[0]
	waitAndAssert(c, defaultReconciliationTimeout, func(c *check.C) (interface{}, check.CommentInterface) {
		if task.NodeID == "" || task.Status.ContainerStatus.ContainerID == "" {
			task = d.GetTask(c, task.ID)
		}
		return task.NodeID != "" && task.Status.ContainerStatus.ContainerID != "", nil
	}, checker.Equals, true)

	// check container mount config
	out, err = s.nodeCmd(c, task.NodeID, "inspect", "--format", "{{json .HostConfig.Mounts}}", task.Status.ContainerStatus.ContainerID)
	c.Assert(err, checker.IsNil, check.Commentf(out))

	var mountConfig []mount.Mount
	c.Assert(json.Unmarshal([]byte(out), &mountConfig), checker.IsNil)
	c.Assert(mountConfig, checker.HasLen, 1)

	c.Assert(mountConfig[0].Source, checker.Equals, "foo")
	c.Assert(mountConfig[0].Target, checker.Equals, "/foo")
	c.Assert(mountConfig[0].Type, checker.Equals, mount.TypeVolume)
	c.Assert(mountConfig[0].VolumeOptions, checker.NotNil)
	c.Assert(mountConfig[0].VolumeOptions.NoCopy, checker.True)

	// check container mounts actual
	out, err = s.nodeCmd(c, task.NodeID, "inspect", "--format", "{{json .Mounts}}", task.Status.ContainerStatus.ContainerID)
	c.Assert(err, checker.IsNil, check.Commentf(out))

	var mounts []types.MountPoint
	c.Assert(json.Unmarshal([]byte(out), &mounts), checker.IsNil)
	c.Assert(mounts, checker.HasLen, 1)

	c.Assert(mounts[0].Type, checker.Equals, mount.TypeVolume)
	c.Assert(mounts[0].Name, checker.Equals, "foo")
	c.Assert(mounts[0].Destination, checker.Equals, "/foo")
	c.Assert(mounts[0].RW, checker.Equals, true)
}

func (s *DockerSwarmSuite) TestServiceCreateWithSecretSimple(c *check.C) {
	d := s.AddDaemon(c, true, true)

	serviceName := "test-service-secret"
	testName := "test_secret"
	id := d.CreateSecret(c, swarm.SecretSpec{
		Annotations: swarm.Annotations{
			Name: testName,
		},
		Data: []byte("TESTINGDATA"),
	})
	c.Assert(id, checker.Not(checker.Equals), "", check.Commentf("secrets: %s", id))

	out, err := d.Cmd("service", "create", "--no-resolve-image", "--name", serviceName, "--secret", testName, "busybox", "top")
	c.Assert(err, checker.IsNil, check.Commentf(out))

	out, err = d.Cmd("service", "inspect", "--format", "{{ json .Spec.TaskTemplate.ContainerSpec.Secrets }}", serviceName)
	c.Assert(err, checker.IsNil)

	var refs []swarm.SecretReference
	c.Assert(json.Unmarshal([]byte(out), &refs), checker.IsNil)
	c.Assert(refs, checker.HasLen, 1)

	c.Assert(refs[0].SecretName, checker.Equals, testName)
	c.Assert(refs[0].File, checker.Not(checker.IsNil))
	c.Assert(refs[0].File.Name, checker.Equals, testName)
	c.Assert(refs[0].File.UID, checker.Equals, "0")
	c.Assert(refs[0].File.GID, checker.Equals, "0")

	out, err = d.Cmd("service", "rm", serviceName)
	c.Assert(err, checker.IsNil, check.Commentf(out))
	d.DeleteSecret(c, testName)
}

func (s *DockerSwarmSuite) TestServiceCreateWithSecretSourceTargetPaths(c *check.C) {
	d := s.AddDaemon(c, true, true)

	testPaths := map[string]string{
		"app":                  "/etc/secret",
		"test_secret":          "test_secret",
		"relative_secret":      "relative/secret",
		"escapes_in_container": "../secret",
	}

	var secretFlags []string

	for testName, testTarget := range testPaths {
		id := d.CreateSecret(c, swarm.SecretSpec{
			Annotations: swarm.Annotations{
				Name: testName,
			},
			Data: []byte("TESTINGDATA " + testName + " " + testTarget),
		})
		c.Assert(id, checker.Not(checker.Equals), "", check.Commentf("secrets: %s", id))

		secretFlags = append(secretFlags, "--secret", fmt.Sprintf("source=%s,target=%s", testName, testTarget))
	}

	serviceName := "svc"
	serviceCmd := []string{"service", "create", "--no-resolve-image", "--name", serviceName}
	serviceCmd = append(serviceCmd, secretFlags...)
	serviceCmd = append(serviceCmd, "busybox", "top")
	out, err := d.Cmd(serviceCmd...)
	c.Assert(err, checker.IsNil, check.Commentf(out))

	out, err = d.Cmd("service", "inspect", "--format", "{{ json .Spec.TaskTemplate.ContainerSpec.Secrets }}", serviceName)
	c.Assert(err, checker.IsNil)

	var refs []swarm.SecretReference
	c.Assert(json.Unmarshal([]byte(out), &refs), checker.IsNil)
	c.Assert(refs, checker.HasLen, len(testPaths))

	var tasks []swarm.Task
	waitAndAssert(c, defaultReconciliationTimeout, func(c *check.C) (interface{}, check.CommentInterface) {
		tasks = d.GetServiceTasks(c, serviceName)
		return len(tasks) > 0, nil
	}, checker.Equals, true)

	task := tasks[0]
	waitAndAssert(c, defaultReconciliationTimeout, func(c *check.C) (interface{}, check.CommentInterface) {
		if task.NodeID == "" || task.Status.ContainerStatus.ContainerID == "" {
			task = d.GetTask(c, task.ID)
		}
		return task.NodeID != "" && task.Status.ContainerStatus.ContainerID != "", nil
	}, checker.Equals, true)

	for testName, testTarget := range testPaths {
		path := testTarget
		if !filepath.IsAbs(path) {
			path = filepath.Join("/run/secrets", path)
		}
		out, err := d.Cmd("exec", task.Status.ContainerStatus.ContainerID, "cat", path)
		c.Assert(err, checker.IsNil)
		c.Assert(out, checker.Equals, "TESTINGDATA "+testName+" "+testTarget)
	}

	out, err = d.Cmd("service", "rm", serviceName)
	c.Assert(err, checker.IsNil, check.Commentf(out))
}

func (s *DockerSwarmSuite) TestServiceCreateWithSecretReferencedTwice(c *check.C) {
	d := s.AddDaemon(c, true, true)

	id := d.CreateSecret(c, swarm.SecretSpec{
		Annotations: swarm.Annotations{
			Name: "mysecret",
		},
		Data: []byte("TESTINGDATA"),
	})
	c.Assert(id, checker.Not(checker.Equals), "", check.Commentf("secrets: %s", id))

	serviceName := "svc"
	out, err := d.Cmd("service", "create", "--no-resolve-image", "--name", serviceName, "--secret", "source=mysecret,target=target1", "--secret", "source=mysecret,target=target2", "busybox", "top")
	c.Assert(err, checker.IsNil, check.Commentf(out))

	out, err = d.Cmd("service", "inspect", "--format", "{{ json .Spec.TaskTemplate.ContainerSpec.Secrets }}", serviceName)
	c.Assert(err, checker.IsNil)

	var refs []swarm.SecretReference
	c.Assert(json.Unmarshal([]byte(out), &refs), checker.IsNil)
	c.Assert(refs, checker.HasLen, 2)

	var tasks []swarm.Task
	waitAndAssert(c, defaultReconciliationTimeout, func(c *check.C) (interface{}, check.CommentInterface) {
		tasks = d.GetServiceTasks(c, serviceName)
		return len(tasks) > 0, nil
	}, checker.Equals, true)

	task := tasks[0]
	waitAndAssert(c, defaultReconciliationTimeout, func(c *check.C) (interface{}, check.CommentInterface) {
		if task.NodeID == "" || task.Status.ContainerStatus.ContainerID == "" {
			task = d.GetTask(c, task.ID)
		}
		return task.NodeID != "" && task.Status.ContainerStatus.ContainerID != "", nil
	}, checker.Equals, true)

	for _, target := range []string{"target1", "target2"} {
		c.Assert(err, checker.IsNil, check.Commentf(out))
		path := filepath.Join("/run/secrets", target)
		out, err := d.Cmd("exec", task.Status.ContainerStatus.ContainerID, "cat", path)
		c.Assert(err, checker.IsNil)
		c.Assert(out, checker.Equals, "TESTINGDATA")
	}

	out, err = d.Cmd("service", "rm", serviceName)
	c.Assert(err, checker.IsNil, check.Commentf(out))
}

func (s *DockerSwarmSuite) TestServiceCreateWithConfigSimple(c *check.C) {
	d := s.AddDaemon(c, true, true)

	serviceName := "test-service-config"
	testName := "test_config"
	id := d.CreateConfig(c, swarm.ConfigSpec{
		Annotations: swarm.Annotations{
			Name: testName,
		},
		Data: []byte("TESTINGDATA"),
	})
	c.Assert(id, checker.Not(checker.Equals), "", check.Commentf("configs: %s", id))

	out, err := d.Cmd("service", "create", "--no-resolve-image", "--name", serviceName, "--config", testName, "busybox", "top")
	c.Assert(err, checker.IsNil, check.Commentf(out))

	out, err = d.Cmd("service", "inspect", "--format", "{{ json .Spec.TaskTemplate.ContainerSpec.Configs }}", serviceName)
	c.Assert(err, checker.IsNil)

	var refs []swarm.ConfigReference
	c.Assert(json.Unmarshal([]byte(out), &refs), checker.IsNil)
	c.Assert(refs, checker.HasLen, 1)

	c.Assert(refs[0].ConfigName, checker.Equals, testName)
	c.Assert(refs[0].File, checker.Not(checker.IsNil))
	c.Assert(refs[0].File.Name, checker.Equals, testName)
	c.Assert(refs[0].File.UID, checker.Equals, "0")
	c.Assert(refs[0].File.GID, checker.Equals, "0")

	out, err = d.Cmd("service", "rm", serviceName)
	c.Assert(err, checker.IsNil, check.Commentf(out))
	d.DeleteConfig(c, testName)
}

func (s *DockerSwarmSuite) TestServiceCreateWithConfigSourceTargetPaths(c *check.C) {
	d := s.AddDaemon(c, true, true)

	testPaths := map[string]string{
		"app":             "/etc/config",
		"test_config":     "test_config",
		"relative_config": "relative/config",
	}

	var configFlags []string

	for testName, testTarget := range testPaths {
		id := d.CreateConfig(c, swarm.ConfigSpec{
			Annotations: swarm.Annotations{
				Name: testName,
			},
			Data: []byte("TESTINGDATA " + testName + " " + testTarget),
		})
		c.Assert(id, checker.Not(checker.Equals), "", check.Commentf("configs: %s", id))

		configFlags = append(configFlags, "--config", fmt.Sprintf("source=%s,target=%s", testName, testTarget))
	}

	serviceName := "svc"
	serviceCmd := []string{"service", "create", "--no-resolve-image", "--name", serviceName}
	serviceCmd = append(serviceCmd, configFlags...)
	serviceCmd = append(serviceCmd, "busybox", "top")
	out, err := d.Cmd(serviceCmd...)
	c.Assert(err, checker.IsNil, check.Commentf(out))

	out, err = d.Cmd("service", "inspect", "--format", "{{ json .Spec.TaskTemplate.ContainerSpec.Configs }}", serviceName)
	c.Assert(err, checker.IsNil)

	var refs []swarm.ConfigReference
	c.Assert(json.Unmarshal([]byte(out), &refs), checker.IsNil)
	c.Assert(refs, checker.HasLen, len(testPaths))

	var tasks []swarm.Task
	waitAndAssert(c, defaultReconciliationTimeout, func(c *check.C) (interface{}, check.CommentInterface) {
		tasks = d.GetServiceTasks(c, serviceName)
		return len(tasks) > 0, nil
	}, checker.Equals, true)

	task := tasks[0]
	waitAndAssert(c, defaultReconciliationTimeout, func(c *check.C) (interface{}, check.CommentInterface) {
		if task.NodeID == "" || task.Status.ContainerStatus.ContainerID == "" {
			task = d.GetTask(c, task.ID)
		}
		return task.NodeID != "" && task.Status.ContainerStatus.ContainerID != "", nil
	}, checker.Equals, true)

	for testName, testTarget := range testPaths {
		path := testTarget
		if !filepath.IsAbs(path) {
			path = filepath.Join("/", path)
		}
		out, err := d.Cmd("exec", task.Status.ContainerStatus.ContainerID, "cat", path)
		c.Assert(err, checker.IsNil)
		c.Assert(out, checker.Equals, "TESTINGDATA "+testName+" "+testTarget)
	}

	out, err = d.Cmd("service", "rm", serviceName)
	c.Assert(err, checker.IsNil, check.Commentf(out))
}

func (s *DockerSwarmSuite) TestServiceCreateWithConfigReferencedTwice(c *check.C) {
	d := s.AddDaemon(c, true, true)

	id := d.CreateConfig(c, swarm.ConfigSpec{
		Annotations: swarm.Annotations{
			Name: "myconfig",
		},
		Data: []byte("TESTINGDATA"),
	})
	c.Assert(id, checker.Not(checker.Equals), "", check.Commentf("configs: %s", id))

	serviceName := "svc"
	out, err := d.Cmd("service", "create", "--no-resolve-image", "--name", serviceName, "--config", "source=myconfig,target=target1", "--config", "source=myconfig,target=target2", "busybox", "top")
	c.Assert(err, checker.IsNil, check.Commentf(out))

	out, err = d.Cmd("service", "inspect", "--format", "{{ json .Spec.TaskTemplate.ContainerSpec.Configs }}", serviceName)
	c.Assert(err, checker.IsNil)

	var refs []swarm.ConfigReference
	c.Assert(json.Unmarshal([]byte(out), &refs), checker.IsNil)
	c.Assert(refs, checker.HasLen, 2)

	var tasks []swarm.Task
	waitAndAssert(c, defaultReconciliationTimeout, func(c *check.C) (interface{}, check.CommentInterface) {
		tasks = d.GetServiceTasks(c, serviceName)
		return len(tasks) > 0, nil
	}, checker.Equals, true)

	task := tasks[0]
	waitAndAssert(c, defaultReconciliationTimeout, func(c *check.C) (interface{}, check.CommentInterface) {
		if task.NodeID == "" || task.Status.ContainerStatus.ContainerID == "" {
			task = d.GetTask(c, task.ID)
		}
		return task.NodeID != "" && task.Status.ContainerStatus.ContainerID != "", nil
	}, checker.Equals, true)

	for _, target := range []string{"target1", "target2"} {
		c.Assert(err, checker.IsNil, check.Commentf(out))
		path := filepath.Join("/", target)
		out, err := d.Cmd("exec", task.Status.ContainerStatus.ContainerID, "cat", path)
		c.Assert(err, checker.IsNil)
		c.Assert(out, checker.Equals, "TESTINGDATA")
	}

	out, err = d.Cmd("service", "rm", serviceName)
	c.Assert(err, checker.IsNil, check.Commentf(out))
}

func (s *DockerSwarmSuite) TestServiceCreateMountTmpfs(c *check.C) {
	d := s.AddDaemon(c, true, true)
	out, err := d.Cmd("service", "create", "--no-resolve-image", "--detach=true", "--mount", "type=tmpfs,target=/foo,tmpfs-size=1MB", "busybox", "sh", "-c", "mount | grep foo; tail -f /dev/null")
	c.Assert(err, checker.IsNil, check.Commentf(out))
	id := strings.TrimSpace(out)

	var tasks []swarm.Task
	waitAndAssert(c, defaultReconciliationTimeout, func(c *check.C) (interface{}, check.CommentInterface) {
		tasks = d.GetServiceTasks(c, id)
		return len(tasks) > 0, nil
	}, checker.Equals, true)

	task := tasks[0]
	waitAndAssert(c, defaultReconciliationTimeout, func(c *check.C) (interface{}, check.CommentInterface) {
		if task.NodeID == "" || task.Status.ContainerStatus.ContainerID == "" {
			task = d.GetTask(c, task.ID)
		}
		return task.NodeID != "" && task.Status.ContainerStatus.ContainerID != "", nil
	}, checker.Equals, true)

	// check container mount config
	out, err = s.nodeCmd(c, task.NodeID, "inspect", "--format", "{{json .HostConfig.Mounts}}", task.Status.ContainerStatus.ContainerID)
	c.Assert(err, checker.IsNil, check.Commentf(out))

	var mountConfig []mount.Mount
	c.Assert(json.Unmarshal([]byte(out), &mountConfig), checker.IsNil)
	c.Assert(mountConfig, checker.HasLen, 1)

	c.Assert(mountConfig[0].Source, checker.Equals, "")
	c.Assert(mountConfig[0].Target, checker.Equals, "/foo")
	c.Assert(mountConfig[0].Type, checker.Equals, mount.TypeTmpfs)
	c.Assert(mountConfig[0].TmpfsOptions, checker.NotNil)
	c.Assert(mountConfig[0].TmpfsOptions.SizeBytes, checker.Equals, int64(1048576))

	// check container mounts actual
	out, err = s.nodeCmd(c, task.NodeID, "inspect", "--format", "{{json .Mounts}}", task.Status.ContainerStatus.ContainerID)
	c.Assert(err, checker.IsNil, check.Commentf(out))

	var mounts []types.MountPoint
	c.Assert(json.Unmarshal([]byte(out), &mounts), checker.IsNil)
	c.Assert(mounts, checker.HasLen, 1)

	c.Assert(mounts[0].Type, checker.Equals, mount.TypeTmpfs)
	c.Assert(mounts[0].Name, checker.Equals, "")
	c.Assert(mounts[0].Destination, checker.Equals, "/foo")
	c.Assert(mounts[0].RW, checker.Equals, true)

	out, err = s.nodeCmd(c, task.NodeID, "logs", task.Status.ContainerStatus.ContainerID)
	c.Assert(err, checker.IsNil, check.Commentf(out))
	c.Assert(strings.TrimSpace(out), checker.HasPrefix, "tmpfs on /foo type tmpfs")
	c.Assert(strings.TrimSpace(out), checker.Contains, "size=1024k")
}

func (s *DockerSwarmSuite) TestServiceCreateWithNetworkAlias(c *check.C) {
	d := s.AddDaemon(c, true, true)
	out, err := d.Cmd("network", "create", "--scope=swarm", "test_swarm_br")
	c.Assert(err, checker.IsNil, check.Commentf(out))

	out, err = d.Cmd("service", "create", "--no-resolve-image", "--detach=true", "--network=name=test_swarm_br,alias=srv_alias", "--name=alias_tst_container", "busybox", "top")
	c.Assert(err, checker.IsNil, check.Commentf(out))
	id := strings.TrimSpace(out)

	var tasks []swarm.Task
	waitAndAssert(c, defaultReconciliationTimeout, func(c *check.C) (interface{}, check.CommentInterface) {
		tasks = d.GetServiceTasks(c, id)
		return len(tasks) > 0, nil
	}, checker.Equals, true)

	task := tasks[0]
	waitAndAssert(c, defaultReconciliationTimeout, func(c *check.C) (interface{}, check.CommentInterface) {
		if task.NodeID == "" || task.Status.ContainerStatus.ContainerID == "" {
			task = d.GetTask(c, task.ID)
		}
		return task.NodeID != "" && task.Status.ContainerStatus.ContainerID != "", nil
	}, checker.Equals, true)

	// check container alias config
	out, err = s.nodeCmd(c, task.NodeID, "inspect", "--format", "{{json .NetworkSettings.Networks.test_swarm_br.Aliases}}", task.Status.ContainerStatus.ContainerID)
	c.Assert(err, checker.IsNil, check.Commentf(out))

	// Make sure the only alias seen is the container-id
	var aliases []string
	c.Assert(json.Unmarshal([]byte(out), &aliases), checker.IsNil)
	c.Assert(aliases, checker.HasLen, 1)

	c.Assert(task.Status.ContainerStatus.ContainerID, checker.Contains, aliases[0])
}

func (s *DockerSwarmSuite) TestServiceCreateWithTemplatingHostname(c *check.C) {
	d := s.AddDaemon(c, true, true)

	out, err := d.Cmd("service", "create", "--no-resolve-image", "--name", "test", "--hostname", "{{.Service.Name}}-{{.Task.Slot}}", "busybox", "top")
	c.Assert(err, checker.IsNil, check.Commentf(out))

	// make sure task has been deployed.
	waitAndAssert(c, defaultReconciliationTimeout, d.CheckActiveContainerCount, checker.Equals, 1)

	containers := d.ActiveContainers()
	out, err = d.Cmd("inspect", "--type", "container", "--format", "{{.Config.Hostname}}", containers[0])
	c.Assert(err, checker.IsNil, check.Commentf(out))
	c.Assert(strings.Split(out, "\n")[0], checker.Equals, "test-1", check.Commentf("hostname with templating invalid"))
}

func (s *DockerSwarmSuite) TestServiceCreateWithGroup(c *check.C) {
	d := s.AddDaemon(c, true, true)

	name := "top"
	out, err := d.Cmd("service", "create", "--no-resolve-image", "--name", name, "--user", "root:root", "--group", "wheel", "--group", "audio", "--group", "staff", "--group", "777", "busybox", "top")
	c.Assert(err, checker.IsNil)
	c.Assert(strings.TrimSpace(out), checker.Not(checker.Equals), "")

	// make sure task has been deployed.
	waitAndAssert(c, defaultReconciliationTimeout, d.CheckActiveContainerCount, checker.Equals, 1)

	out, err = d.Cmd("ps", "-q")
	c.Assert(err, checker.IsNil)
	c.Assert(strings.TrimSpace(out), checker.Not(checker.Equals), "")

	container := strings.TrimSpace(out)

	out, err = d.Cmd("exec", container, "id")
	c.Assert(err, checker.IsNil)
	c.Assert(strings.TrimSpace(out), checker.Equals, "uid=0(root) gid=0(root) groups=10(wheel),29(audio),50(staff),777")
}

func (s *DockerSwarmSuite) TestServiceCreateWithNoIngressNetwork(c *check.C) {
	d := s.AddDaemon(c, true, true)

	// Remove ingress network
	out, _, err := testutil.RunCommandPipelineWithOutput(
		exec.Command("echo", "Y"),
		exec.Command("docker", "-H", d.Sock(), "network", "rm", "ingress"),
	)
	c.Assert(err, checker.IsNil, check.Commentf(out))

	// Create a overlay network and launch a service on it
	// Make sure nothing panics because ingress network is missing
	out, err = d.Cmd("network", "create", "-d", "overlay", "another-network")
	c.Assert(err, checker.IsNil, check.Commentf(out))
	out, err = d.Cmd("service", "create", "--no-resolve-image", "--name", "srv4", "--network", "another-network", "busybox", "top")
	c.Assert(err, checker.IsNil, check.Commentf(out))
}

// Test case for #24712
func (s *DockerSwarmSuite) TestServiceCreateWithEnvFile(c *check.C) {
	d := s.AddDaemon(c, true, true)

	path := filepath.Join(d.Folder, "env.txt")
	err := ioutil.WriteFile(path, []byte("VAR1=A\nVAR2=A\n"), 0644)
	c.Assert(err, checker.IsNil)

	name := "worker"
	out, err := d.Cmd("service", "create", "--no-resolve-image", "--env-file", path, "--env", "VAR1=B", "--env", "VAR1=C", "--env", "VAR2=", "--env", "VAR2", "--name", name, "busybox", "top")
	c.Assert(err, checker.IsNil)
	c.Assert(strings.TrimSpace(out), checker.Not(checker.Equals), "")

	// The complete env is [VAR1=A VAR2=A VAR1=B VAR1=C VAR2= VAR2] and duplicates will be removed => [VAR1=C VAR2]
	out, err = d.Cmd("inspect", "--format", "{{ .Spec.TaskTemplate.ContainerSpec.Env }}", name)
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Contains, "[VAR1=C VAR2]")
}

func (s *DockerSwarmSuite) TestServiceCreateWithTTY(c *check.C) {
	d := s.AddDaemon(c, true, true)

	name := "top"

	ttyCheck := "if [ -t 0 ]; then echo TTY > /status && top; else echo none > /status && top; fi"

	// Without --tty
	expectedOutput := "none"
	out, err := d.Cmd("service", "create", "--no-resolve-image", "--name", name, "busybox", "sh", "-c", ttyCheck)
	c.Assert(err, checker.IsNil)

	// Make sure task has been deployed.
	waitAndAssert(c, defaultReconciliationTimeout, d.CheckActiveContainerCount, checker.Equals, 1)

	// We need to get the container id.
	out, err = d.Cmd("ps", "-a", "-q", "--no-trunc")
	c.Assert(err, checker.IsNil)
	id := strings.TrimSpace(out)

	out, err = d.Cmd("exec", id, "cat", "/status")
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Contains, expectedOutput, check.Commentf("Expected '%s', but got %q", expectedOutput, out))

	// Remove service
	out, err = d.Cmd("service", "rm", name)
	c.Assert(err, checker.IsNil)
	// Make sure container has been destroyed.
	waitAndAssert(c, defaultReconciliationTimeout, d.CheckActiveContainerCount, checker.Equals, 0)

	// With --tty
	expectedOutput = "TTY"
	out, err = d.Cmd("service", "create", "--no-resolve-image", "--name", name, "--tty", "busybox", "sh", "-c", ttyCheck)
	c.Assert(err, checker.IsNil)

	// Make sure task has been deployed.
	waitAndAssert(c, defaultReconciliationTimeout, d.CheckActiveContainerCount, checker.Equals, 1)

	// We need to get the container id.
	out, err = d.Cmd("ps", "-a", "-q", "--no-trunc")
	c.Assert(err, checker.IsNil)
	id = strings.TrimSpace(out)

	out, err = d.Cmd("exec", id, "cat", "/status")
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Contains, expectedOutput, check.Commentf("Expected '%s', but got %q", expectedOutput, out))
}

func (s *DockerSwarmSuite) TestServiceCreateWithDNSConfig(c *check.C) {
	d := s.AddDaemon(c, true, true)

	// Create a service
	name := "top"
	_, err := d.Cmd("service", "create", "--no-resolve-image", "--name", name, "--dns=1.2.3.4", "--dns-search=example.com", "--dns-option=timeout:3", "busybox", "top")
	c.Assert(err, checker.IsNil)

	// Make sure task has been deployed.
	waitAndAssert(c, defaultReconciliationTimeout, d.CheckActiveContainerCount, checker.Equals, 1)

	// We need to get the container id.
	out, err := d.Cmd("ps", "-a", "-q", "--no-trunc")
	c.Assert(err, checker.IsNil)
	id := strings.TrimSpace(out)

	// Compare against expected output.
	expectedOutput1 := "nameserver 1.2.3.4"
	expectedOutput2 := "search example.com"
	expectedOutput3 := "options timeout:3"
	out, err = d.Cmd("exec", id, "cat", "/etc/resolv.conf")
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Contains, expectedOutput1, check.Commentf("Expected '%s', but got %q", expectedOutput1, out))
	c.Assert(out, checker.Contains, expectedOutput2, check.Commentf("Expected '%s', but got %q", expectedOutput2, out))
	c.Assert(out, checker.Contains, expectedOutput3, check.Commentf("Expected '%s', but got %q", expectedOutput3, out))
}

func (s *DockerSwarmSuite) TestServiceCreateWithExtraHosts(c *check.C) {
	d := s.AddDaemon(c, true, true)

	// Create a service
	name := "top"
	_, err := d.Cmd("service", "create", "--no-resolve-image", "--name", name, "--host=example.com:1.2.3.4", "busybox", "top")
	c.Assert(err, checker.IsNil)

	// Make sure task has been deployed.
	waitAndAssert(c, defaultReconciliationTimeout, d.CheckActiveContainerCount, checker.Equals, 1)

	// We need to get the container id.
	out, err := d.Cmd("ps", "-a", "-q", "--no-trunc")
	c.Assert(err, checker.IsNil)
	id := strings.TrimSpace(out)

	// Compare against expected output.
	expectedOutput := "1.2.3.4\texample.com"
	out, err = d.Cmd("exec", id, "cat", "/etc/hosts")
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Contains, expectedOutput, check.Commentf("Expected '%s', but got %q", expectedOutput, out))
}

func (s *DockerTrustedSwarmSuite) TestTrustedServiceCreate(c *check.C) {
	d := s.swarmSuite.AddDaemon(c, true, true)

	// Attempt creating a service from an image that is known to notary.
	repoName := s.trustSuite.setupTrustedImage(c, "trusted-pull")

	name := "trusted"
	cli.Docker(cli.Args("-D", "service", "create", "--no-resolve-image", "--name", name, repoName, "top"), trustedCmd, cli.Daemon(d.Daemon)).Assert(c, icmd.Expected{
		Err: "resolved image tag to",
	})

	out, err := d.Cmd("service", "inspect", "--pretty", name)
	c.Assert(err, checker.IsNil, check.Commentf(out))
	c.Assert(out, checker.Contains, repoName+"@", check.Commentf(out))

	// Try trusted service create on an untrusted tag.

	repoName = fmt.Sprintf("%v/untrustedservicecreate/createtest:latest", privateRegistryURL)
	// tag the image and upload it to the private registry
	cli.DockerCmd(c, "tag", "busybox", repoName)
	cli.DockerCmd(c, "push", repoName)
	cli.DockerCmd(c, "rmi", repoName)

	name = "untrusted"
	cli.Docker(cli.Args("service", "create", "--no-resolve-image", "--name", name, repoName, "top"), trustedCmd, cli.Daemon(d.Daemon)).Assert(c, icmd.Expected{
		ExitCode: 1,
		Err:      "Error: remote trust data does not exist",
	})

	out, err = d.Cmd("service", "inspect", "--pretty", name)
	c.Assert(err, checker.NotNil, check.Commentf(out))
}

func (s *DockerSwarmSuite) TestServiceCreateWithPublishDuplicatePorts(c *check.C) {
	d := s.AddDaemon(c, true, true)

	out, err := d.Cmd("service", "create", "--no-resolve-image", "--detach=true", "--publish", "5005:80", "--publish", "5006:80", "--publish", "80", "--publish", "80", "busybox", "top")
	c.Assert(err, check.IsNil, check.Commentf(out))
	id := strings.TrimSpace(out)

	// make sure task has been deployed.
	waitAndAssert(c, defaultReconciliationTimeout, d.CheckActiveContainerCount, checker.Equals, 1)

	// Total len = 4, with 2 dynamic ports and 2 non-dynamic ports
	// Dynamic ports are likely to be 30000 and 30001 but doesn't matter
	out, err = d.Cmd("service", "inspect", "--format", "{{.Endpoint.Ports}} len={{len .Endpoint.Ports}}", id)
	c.Assert(err, check.IsNil, check.Commentf(out))
	c.Assert(out, checker.Contains, "len=4")
	c.Assert(out, checker.Contains, "{ tcp 80 5005 ingress}")
	c.Assert(out, checker.Contains, "{ tcp 80 5006 ingress}")
}
