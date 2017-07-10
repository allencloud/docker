package main

import (
	"strings"

	"github.com/docker/docker/integration-cli/checker"
	"github.com/go-check/check"
)

func (s *DockerSwarmSuite) TestNodeTaskListFilter(c *check.C) {
	d := s.AddDaemon(c, true, true)

	name := "redis-cluster-md5"
	out, err := d.Cmd("service", "create", "--no-resolve-image", "--name", name, "--replicas=3", "busybox", "top")
	c.Assert(err, checker.IsNil)
	c.Assert(strings.TrimSpace(out), checker.Not(checker.Equals), "")

	// make sure task has been deployed.
	waitAndAssert(c, defaultReconciliationTimeout, d.CheckActiveContainerCount, checker.Equals, 3)

	filter := "name=redis-cluster"

	out, err = d.Cmd("node", "ps", "--filter", filter, "self")
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Contains, name+".1")
	c.Assert(out, checker.Contains, name+".2")
	c.Assert(out, checker.Contains, name+".3")

	out, err = d.Cmd("node", "ps", "--filter", "name=none", "self")
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Not(checker.Contains), name+".1")
	c.Assert(out, checker.Not(checker.Contains), name+".2")
	c.Assert(out, checker.Not(checker.Contains), name+".3")
}
