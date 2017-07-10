// +build !windows

package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/integration-cli/checker"
	"github.com/docker/docker/integration-cli/cli"
	icmd "github.com/docker/docker/pkg/testutil/cmd"
	"github.com/go-check/check"
)

func (s *DockerSwarmSuite) TestServiceUpdatePort(c *check.C) {
	d := s.AddDaemon(c, true, true)

	serviceName := "TestServiceUpdatePort"
	serviceArgs := append([]string{"service", "create", "--no-resolve-image", "--name", serviceName, "-p", "8080:8081", defaultSleepImage}, sleepCommandForDaemonPlatform()...)

	// Create a service with a port mapping of 8080:8081.
	out, err := d.Cmd(serviceArgs...)
	c.Assert(err, checker.IsNil)
	waitAndAssert(c, defaultReconciliationTimeout, d.CheckActiveContainerCount, checker.Equals, 1)

	// Update the service: changed the port mapping from 8080:8081 to 8082:8083.
	_, err = d.Cmd("service", "update", "--publish-add", "8082:8083", "--publish-rm", "8081", serviceName)
	c.Assert(err, checker.IsNil)

	// Inspect the service and verify port mapping
	expected := []swarm.PortConfig{
		{
			Protocol:      "tcp",
			PublishedPort: 8082,
			TargetPort:    8083,
			PublishMode:   "ingress",
		},
	}

	out, err = d.Cmd("service", "inspect", "--format", "{{ json .Spec.EndpointSpec.Ports }}", serviceName)
	c.Assert(err, checker.IsNil)

	var portConfig []swarm.PortConfig
	if err := json.Unmarshal([]byte(out), &portConfig); err != nil {
		c.Fatalf("invalid JSON in inspect result: %v (%s)", err, out)
	}
	c.Assert(portConfig, checker.DeepEquals, expected)
}

func (s *DockerSwarmSuite) TestServiceUpdateLabel(c *check.C) {
	d := s.AddDaemon(c, true, true)
	out, err := d.Cmd("service", "create", "--no-resolve-image", "--name=test", "busybox", "top")
	c.Assert(err, checker.IsNil, check.Commentf(out))
	service := d.GetService(c, "test")
	c.Assert(service.Spec.Labels, checker.HasLen, 0)

	// add label to empty set
	out, err = d.Cmd("service", "update", "test", "--label-add", "foo=bar")
	c.Assert(err, checker.IsNil, check.Commentf(out))
	service = d.GetService(c, "test")
	c.Assert(service.Spec.Labels, checker.HasLen, 1)
	c.Assert(service.Spec.Labels["foo"], checker.Equals, "bar")

	// add label to non-empty set
	out, err = d.Cmd("service", "update", "test", "--label-add", "foo2=bar")
	c.Assert(err, checker.IsNil, check.Commentf(out))
	service = d.GetService(c, "test")
	c.Assert(service.Spec.Labels, checker.HasLen, 2)
	c.Assert(service.Spec.Labels["foo2"], checker.Equals, "bar")

	out, err = d.Cmd("service", "update", "test", "--label-rm", "foo2")
	c.Assert(err, checker.IsNil, check.Commentf(out))
	service = d.GetService(c, "test")
	c.Assert(service.Spec.Labels, checker.HasLen, 1)
	c.Assert(service.Spec.Labels["foo2"], checker.Equals, "")

	out, err = d.Cmd("service", "update", "test", "--label-rm", "foo")
	c.Assert(err, checker.IsNil, check.Commentf(out))
	service = d.GetService(c, "test")
	c.Assert(service.Spec.Labels, checker.HasLen, 0)
	c.Assert(service.Spec.Labels["foo"], checker.Equals, "")

	// now make sure we can add again
	out, err = d.Cmd("service", "update", "test", "--label-add", "foo=bar")
	c.Assert(err, checker.IsNil, check.Commentf(out))
	service = d.GetService(c, "test")
	c.Assert(service.Spec.Labels, checker.HasLen, 1)
	c.Assert(service.Spec.Labels["foo"], checker.Equals, "bar")
}

func (s *DockerSwarmSuite) TestServiceUpdateSecrets(c *check.C) {
	d := s.AddDaemon(c, true, true)
	testName := "test_secret"
	id := d.CreateSecret(c, swarm.SecretSpec{
		Annotations: swarm.Annotations{
			Name: testName,
		},
		Data: []byte("TESTINGDATA"),
	})
	c.Assert(id, checker.Not(checker.Equals), "", check.Commentf("secrets: %s", id))
	testTarget := "testing"
	serviceName := "test"

	out, err := d.Cmd("service", "create", "--no-resolve-image", "--name", serviceName, "busybox", "top")
	c.Assert(err, checker.IsNil, check.Commentf(out))

	// add secret
	out, err = d.CmdRetryOutOfSequence("service", "update", "test", "--secret-add", fmt.Sprintf("source=%s,target=%s", testName, testTarget))
	c.Assert(err, checker.IsNil, check.Commentf(out))

	out, err = d.Cmd("service", "inspect", "--format", "{{ json .Spec.TaskTemplate.ContainerSpec.Secrets }}", serviceName)
	c.Assert(err, checker.IsNil)

	var refs []swarm.SecretReference
	c.Assert(json.Unmarshal([]byte(out), &refs), checker.IsNil)
	c.Assert(refs, checker.HasLen, 1)

	c.Assert(refs[0].SecretName, checker.Equals, testName)
	c.Assert(refs[0].File, checker.Not(checker.IsNil))
	c.Assert(refs[0].File.Name, checker.Equals, testTarget)

	// remove
	out, err = d.CmdRetryOutOfSequence("service", "update", "test", "--secret-rm", testName)
	c.Assert(err, checker.IsNil, check.Commentf(out))

	out, err = d.Cmd("service", "inspect", "--format", "{{ json .Spec.TaskTemplate.ContainerSpec.Secrets }}", serviceName)
	c.Assert(err, checker.IsNil)

	c.Assert(json.Unmarshal([]byte(out), &refs), checker.IsNil)
	c.Assert(refs, checker.HasLen, 0)
}

func (s *DockerSwarmSuite) TestServiceUpdateConfigs(c *check.C) {
	d := s.AddDaemon(c, true, true)
	testName := "test_config"
	id := d.CreateConfig(c, swarm.ConfigSpec{
		Annotations: swarm.Annotations{
			Name: testName,
		},
		Data: []byte("TESTINGDATA"),
	})
	c.Assert(id, checker.Not(checker.Equals), "", check.Commentf("configs: %s", id))
	testTarget := "/testing"
	serviceName := "test"

	out, err := d.Cmd("service", "create", "--no-resolve-image", "--name", serviceName, "busybox", "top")
	c.Assert(err, checker.IsNil, check.Commentf(out))

	// add config
	out, err = d.CmdRetryOutOfSequence("service", "update", "test", "--config-add", fmt.Sprintf("source=%s,target=%s", testName, testTarget))
	c.Assert(err, checker.IsNil, check.Commentf(out))

	out, err = d.Cmd("service", "inspect", "--format", "{{ json .Spec.TaskTemplate.ContainerSpec.Configs }}", serviceName)
	c.Assert(err, checker.IsNil)

	var refs []swarm.ConfigReference
	c.Assert(json.Unmarshal([]byte(out), &refs), checker.IsNil)
	c.Assert(refs, checker.HasLen, 1)

	c.Assert(refs[0].ConfigName, checker.Equals, testName)
	c.Assert(refs[0].File, checker.Not(checker.IsNil))
	c.Assert(refs[0].File.Name, checker.Equals, testTarget)

	// remove
	out, err = d.CmdRetryOutOfSequence("service", "update", "test", "--config-rm", testName)
	c.Assert(err, checker.IsNil, check.Commentf(out))

	out, err = d.Cmd("service", "inspect", "--format", "{{ json .Spec.TaskTemplate.ContainerSpec.Configs }}", serviceName)
	c.Assert(err, checker.IsNil)

	c.Assert(json.Unmarshal([]byte(out), &refs), checker.IsNil)
	c.Assert(refs, checker.HasLen, 0)
}

func (s *DockerSwarmSuite) TestServiceUpdateTTY(c *check.C) {
	d := s.AddDaemon(c, true, true)

	// Create a service
	name := "top"
	_, err := d.Cmd("service", "create", "--no-resolve-image", "--name", name, "busybox", "top")
	c.Assert(err, checker.IsNil)

	// Make sure task has been deployed.
	waitAndAssert(c, defaultReconciliationTimeout, d.CheckActiveContainerCount, checker.Equals, 1)

	out, err := d.Cmd("service", "inspect", "--format", "{{ .Spec.TaskTemplate.ContainerSpec.TTY }}", name)
	c.Assert(err, checker.IsNil)
	c.Assert(strings.TrimSpace(out), checker.Equals, "false")

	_, err = d.Cmd("service", "update", "--tty", name)
	c.Assert(err, checker.IsNil)

	out, err = d.Cmd("service", "inspect", "--format", "{{ .Spec.TaskTemplate.ContainerSpec.TTY }}", name)
	c.Assert(err, checker.IsNil)
	c.Assert(strings.TrimSpace(out), checker.Equals, "true")
}

func (s *DockerSwarmSuite) TestServiceUpdateNetwork(c *check.C) {
	d := s.AddDaemon(c, true, true)

	result := icmd.RunCmd(d.Command("network", "create", "-d", "overlay", "foo"))
	result.Assert(c, icmd.Success)
	fooNetwork := strings.TrimSpace(string(result.Combined()))

	result = icmd.RunCmd(d.Command("network", "create", "-d", "overlay", "bar"))
	result.Assert(c, icmd.Success)
	barNetwork := strings.TrimSpace(string(result.Combined()))

	result = icmd.RunCmd(d.Command("network", "create", "-d", "overlay", "baz"))
	result.Assert(c, icmd.Success)
	bazNetwork := strings.TrimSpace(string(result.Combined()))

	// Create a service
	name := "top"
	result = icmd.RunCmd(d.Command("service", "create", "--no-resolve-image", "--network", "foo", "--network", "bar", "--name", name, "busybox", "top"))
	result.Assert(c, icmd.Success)

	// Make sure task has been deployed.
	waitAndAssert(c, defaultReconciliationTimeout, d.CheckRunningTaskNetworks, checker.DeepEquals,
		map[string]int{fooNetwork: 1, barNetwork: 1})

	// Remove a network
	result = icmd.RunCmd(d.Command("service", "update", "--network-rm", "foo", name))
	result.Assert(c, icmd.Success)

	waitAndAssert(c, defaultReconciliationTimeout, d.CheckRunningTaskNetworks, checker.DeepEquals,
		map[string]int{barNetwork: 1})

	// Add a network
	result = icmd.RunCmd(d.Command("service", "update", "--network-add", "baz", name))
	result.Assert(c, icmd.Success)

	waitAndAssert(c, defaultReconciliationTimeout, d.CheckRunningTaskNetworks, checker.DeepEquals,
		map[string]int{barNetwork: 1, bazNetwork: 1})
}

func (s *DockerSwarmSuite) TestServiceUpdateDNSConfig(c *check.C) {
	d := s.AddDaemon(c, true, true)

	// Create a service
	name := "top"
	_, err := d.Cmd("service", "create", "--no-resolve-image", "--name", name, "busybox", "top")
	c.Assert(err, checker.IsNil)

	// Make sure task has been deployed.
	waitAndAssert(c, defaultReconciliationTimeout, d.CheckActiveContainerCount, checker.Equals, 1)

	_, err = d.Cmd("service", "update", "--dns-add=1.2.3.4", "--dns-search-add=example.com", "--dns-option-add=timeout:3", name)
	c.Assert(err, checker.IsNil)

	out, err := d.Cmd("service", "inspect", "--format", "{{ .Spec.TaskTemplate.ContainerSpec.DNSConfig }}", name)
	c.Assert(err, checker.IsNil)
	c.Assert(strings.TrimSpace(out), checker.Equals, "{[1.2.3.4] [example.com] [timeout:3]}")
}

func (s *DockerTrustedSwarmSuite) TestTrustedServiceUpdate(c *check.C) {
	d := s.swarmSuite.AddDaemon(c, true, true)

	// Attempt creating a service from an image that is known to notary.
	repoName := s.trustSuite.setupTrustedImage(c, "trusted-pull")

	name := "myservice"

	// Create a service without content trust
	cli.Docker(cli.Args("service", "create", "--no-resolve-image", "--name", name, repoName, "top"), cli.Daemon(d.Daemon)).Assert(c, icmd.Success)

	result := cli.Docker(cli.Args("service", "inspect", "--pretty", name), cli.Daemon(d.Daemon))
	c.Assert(result.Error, checker.IsNil, check.Commentf(result.Combined()))
	// Daemon won't insert the digest because this is disabled by
	// DOCKER_SERVICE_PREFER_OFFLINE_IMAGE.
	c.Assert(result.Combined(), check.Not(checker.Contains), repoName+"@", check.Commentf(result.Combined()))

	cli.Docker(cli.Args("-D", "service", "update", "--no-resolve-image", "--image", repoName, name), trustedCmd, cli.Daemon(d.Daemon)).Assert(c, icmd.Expected{
		Err: "resolved image tag to",
	})

	cli.Docker(cli.Args("service", "inspect", "--pretty", name), cli.Daemon(d.Daemon)).Assert(c, icmd.Expected{
		Out: repoName + "@",
	})

	// Try trusted service update on an untrusted tag.

	repoName = fmt.Sprintf("%v/untrustedservicecreate/createtest:latest", privateRegistryURL)
	// tag the image and upload it to the private registry
	cli.DockerCmd(c, "tag", "busybox", repoName)
	cli.DockerCmd(c, "push", repoName)
	cli.DockerCmd(c, "rmi", repoName)

	cli.Docker(cli.Args("service", "update", "--no-resolve-image", "--image", repoName, name), trustedCmd, cli.Daemon(d.Daemon)).Assert(c, icmd.Expected{
		ExitCode: 1,
		Err:      "Error: remote trust data does not exist",
	})
}

// Test case for #25375
func (s *DockerSwarmSuite) TestServiceUpdatePublishAdd(c *check.C) {
	d := s.AddDaemon(c, true, true)

	name := "top"
	out, err := d.Cmd("service", "create", "--no-resolve-image", "--name", name, "--label", "x=y", "busybox", "top")
	c.Assert(err, checker.IsNil)
	c.Assert(strings.TrimSpace(out), checker.Not(checker.Equals), "")

	out, err = d.Cmd("service", "update", "--publish-add", "80:80", name)
	c.Assert(err, checker.IsNil)

	out, err = d.CmdRetryOutOfSequence("service", "update", "--publish-add", "80:80", name)
	c.Assert(err, checker.IsNil)

	out, err = d.CmdRetryOutOfSequence("service", "update", "--publish-add", "80:80", "--publish-add", "80:20", name)
	c.Assert(err, checker.NotNil)

	out, err = d.Cmd("service", "inspect", "--format", "{{ .Spec.EndpointSpec.Ports }}", name)
	c.Assert(err, checker.IsNil)
	c.Assert(strings.TrimSpace(out), checker.Equals, "[{ tcp 80 80 ingress}]")
}

func (s *DockerSwarmSuite) TestServiceUpdateWithStopSignal(c *check.C) {
	testRequires(c, DaemonIsLinux, UserNamespaceROMount)

	d := s.AddDaemon(c, true, true)

	out, err := d.Cmd("service", "create", "--no-resolve-image", "--name", "top", "--stop-signal=SIGHUP", "busybox", "top")
	c.Assert(err, checker.IsNil, check.Commentf(out))

	// make sure task has been deployed.
	waitAndAssert(c, defaultReconciliationTimeout, d.CheckActiveContainerCount, checker.Equals, 1)

	out, err = d.Cmd("service", "inspect", "--format", "{{ .Spec.TaskTemplate.ContainerSpec.StopSignal }}", "top")
	c.Assert(err, checker.IsNil, check.Commentf(out))
	c.Assert(strings.TrimSpace(out), checker.Equals, "SIGHUP")

	containers := d.ActiveContainers()
	out, err = d.Cmd("inspect", "--type", "container", "--format", "{{.Config.StopSignal}}", containers[0])
	c.Assert(err, checker.IsNil, check.Commentf(out))
	c.Assert(strings.TrimSpace(out), checker.Equals, "SIGHUP")

	out, err = d.Cmd("service", "update", "--stop-signal=SIGUSR1", "top")
	c.Assert(err, checker.IsNil, check.Commentf(out))

	out, err = d.Cmd("service", "inspect", "--format", "{{ .Spec.TaskTemplate.ContainerSpec.StopSignal }}", "top")
	c.Assert(err, checker.IsNil, check.Commentf(out))
	c.Assert(strings.TrimSpace(out), checker.Equals, "SIGUSR1")
}
