package main

import (
	"github.com/docker/docker/integration-cli/checker"
	"github.com/go-check/check"
)

func (s *DockerSwarmSuite) TestServiceInspectPretty(c *check.C) {
	d := s.AddDaemon(c, true, true)

	name := "top"
	out, err := d.Cmd("service", "create", "--no-resolve-image", "--name", name, "--limit-cpu=0.5", "busybox", "top")
	c.Assert(err, checker.IsNil, check.Commentf(out))

	expectedOutput := `
Resources:
 Limits:
  CPU:		0.5`
	out, err = d.Cmd("service", "inspect", "--pretty", name)
	c.Assert(err, checker.IsNil, check.Commentf(out))
	c.Assert(out, checker.Contains, expectedOutput, check.Commentf(out))
}
