package main

import (
	"strings"

	"github.com/docker/docker/integration-cli/checker"
	"github.com/go-check/check"
)

// Test case for #24270
func (s *DockerSwarmSuite) TestServiceListFilter(c *check.C) {
	d := s.AddDaemon(c, true, true)

	name1 := "redis-cluster-md5"
	name2 := "redis-cluster"
	name3 := "other-cluster"
	out, err := d.Cmd("service", "create", "--no-resolve-image", "--name", name1, "busybox", "top")
	c.Assert(err, checker.IsNil)
	c.Assert(strings.TrimSpace(out), checker.Not(checker.Equals), "")

	out, err = d.Cmd("service", "create", "--no-resolve-image", "--name", name2, "busybox", "top")
	c.Assert(err, checker.IsNil)
	c.Assert(strings.TrimSpace(out), checker.Not(checker.Equals), "")

	out, err = d.Cmd("service", "create", "--no-resolve-image", "--name", name3, "busybox", "top")
	c.Assert(err, checker.IsNil)
	c.Assert(strings.TrimSpace(out), checker.Not(checker.Equals), "")

	filter1 := "name=redis-cluster-md5"
	filter2 := "name=redis-cluster"

	// We search checker.Contains with `name+" "` to prevent prefix only.
	out, err = d.Cmd("service", "ls", "--filter", filter1)
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Contains, name1+" ")
	c.Assert(out, checker.Not(checker.Contains), name2+" ")
	c.Assert(out, checker.Not(checker.Contains), name3+" ")

	out, err = d.Cmd("service", "ls", "--filter", filter2)
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Contains, name1+" ")
	c.Assert(out, checker.Contains, name2+" ")
	c.Assert(out, checker.Not(checker.Contains), name3+" ")

	out, err = d.Cmd("service", "ls")
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Contains, name1+" ")
	c.Assert(out, checker.Contains, name2+" ")
	c.Assert(out, checker.Contains, name3+" ")
}

func (s *DockerSwarmSuite) TestServiceLsFilterMode(c *check.C) {
	d := s.AddDaemon(c, true, true)

	out, err := d.Cmd("service", "create", "--no-resolve-image", "--name", "top1", "busybox", "top")
	c.Assert(err, checker.IsNil, check.Commentf(out))
	c.Assert(strings.TrimSpace(out), checker.Not(checker.Equals), "")

	out, err = d.Cmd("service", "create", "--no-resolve-image", "--name", "top2", "--mode=global", "busybox", "top")
	c.Assert(err, checker.IsNil, check.Commentf(out))
	c.Assert(strings.TrimSpace(out), checker.Not(checker.Equals), "")

	// make sure task has been deployed.
	waitAndAssert(c, defaultReconciliationTimeout, d.CheckActiveContainerCount, checker.Equals, 2)

	out, err = d.Cmd("service", "ls")
	c.Assert(err, checker.IsNil, check.Commentf(out))
	c.Assert(out, checker.Contains, "top1")
	c.Assert(out, checker.Contains, "top2")
	c.Assert(out, checker.Not(checker.Contains), "localnet")

	out, err = d.Cmd("service", "ls", "--filter", "mode=global")
	c.Assert(out, checker.Not(checker.Contains), "top1")
	c.Assert(out, checker.Contains, "top2")
	c.Assert(err, checker.IsNil, check.Commentf(out))

	out, err = d.Cmd("service", "ls", "--filter", "mode=replicated")
	c.Assert(err, checker.IsNil, check.Commentf(out))
	c.Assert(out, checker.Contains, "top1")
	c.Assert(out, checker.Not(checker.Contains), "top2")
}
