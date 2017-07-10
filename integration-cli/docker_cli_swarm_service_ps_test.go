package main

import (
	"strings"

	"github.com/docker/docker/integration-cli/checker"
	"github.com/go-check/check"
)

// Test case for #24108, also the case from:
// https://github.com/docker/docker/pull/24620#issuecomment-233715656
func (s *DockerSwarmSuite) TestServicePsFilter(c *check.C) {
	d := s.AddDaemon(c, true, true)

	name := "redis-cluster-md5"
	out, err := d.Cmd("service", "create", "--no-resolve-image", "--name", name, "--replicas=3", "busybox", "top")
	c.Assert(err, checker.IsNil)
	c.Assert(strings.TrimSpace(out), checker.Not(checker.Equals), "")

	filter := "name=redis-cluster"

	checkNumTasks := func(*check.C) (interface{}, check.CommentInterface) {
		out, err := d.Cmd("service", "ps", "--filter", filter, name)
		c.Assert(err, checker.IsNil)
		return len(strings.Split(out, "\n")) - 2, nil // includes header and nl in last line
	}

	// wait until all tasks have been created
	waitAndAssert(c, defaultReconciliationTimeout, checkNumTasks, checker.Equals, 3)

	out, err = d.Cmd("service", "ps", "--filter", filter, name)
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Contains, name+".1")
	c.Assert(out, checker.Contains, name+".2")
	c.Assert(out, checker.Contains, name+".3")

	out, err = d.Cmd("service", "ps", "--filter", "name="+name+".1", name)
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Contains, name+".1")
	c.Assert(out, checker.Not(checker.Contains), name+".2")
	c.Assert(out, checker.Not(checker.Contains), name+".3")

	out, err = d.Cmd("service", "ps", "--filter", "name=none", name)
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Not(checker.Contains), name+".1")
	c.Assert(out, checker.Not(checker.Contains), name+".2")
	c.Assert(out, checker.Not(checker.Contains), name+".3")

	name = "redis-cluster-sha1"
	out, err = d.Cmd("service", "create", "--no-resolve-image", "--name", name, "--mode=global", "busybox", "top")
	c.Assert(err, checker.IsNil)
	c.Assert(strings.TrimSpace(out), checker.Not(checker.Equals), "")

	waitAndAssert(c, defaultReconciliationTimeout, checkNumTasks, checker.Equals, 1)

	filter = "name=redis-cluster"
	out, err = d.Cmd("service", "ps", "--filter", filter, name)
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Contains, name)

	out, err = d.Cmd("service", "ps", "--filter", "name="+name, name)
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Contains, name)

	out, err = d.Cmd("service", "ps", "--filter", "name=none", name)
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Not(checker.Contains), name)
}

func (s *DockerSwarmSuite) TestServicePsMultipleServiceIDs(c *check.C) {
	d := s.AddDaemon(c, true, true)

	name1 := "top1"
	out, err := d.Cmd("service", "create", "--no-resolve-image", "--detach=true", "--name", name1, "--replicas=3", "busybox", "top")
	c.Assert(err, checker.IsNil)
	c.Assert(strings.TrimSpace(out), checker.Not(checker.Equals), "")
	id1 := strings.TrimSpace(out)

	name2 := "top2"
	out, err = d.Cmd("service", "create", "--no-resolve-image", "--detach=true", "--name", name2, "--replicas=3", "busybox", "top")
	c.Assert(err, checker.IsNil)
	c.Assert(strings.TrimSpace(out), checker.Not(checker.Equals), "")
	id2 := strings.TrimSpace(out)

	// make sure task has been deployed.
	waitAndAssert(c, defaultReconciliationTimeout, d.CheckActiveContainerCount, checker.Equals, 6)

	out, err = d.Cmd("service", "ps", name1)
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Contains, name1+".1")
	c.Assert(out, checker.Contains, name1+".2")
	c.Assert(out, checker.Contains, name1+".3")
	c.Assert(out, checker.Not(checker.Contains), name2+".1")
	c.Assert(out, checker.Not(checker.Contains), name2+".2")
	c.Assert(out, checker.Not(checker.Contains), name2+".3")

	out, err = d.Cmd("service", "ps", name1, name2)
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Contains, name1+".1")
	c.Assert(out, checker.Contains, name1+".2")
	c.Assert(out, checker.Contains, name1+".3")
	c.Assert(out, checker.Contains, name2+".1")
	c.Assert(out, checker.Contains, name2+".2")
	c.Assert(out, checker.Contains, name2+".3")

	// Name Prefix
	out, err = d.Cmd("service", "ps", "to")
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Contains, name1+".1")
	c.Assert(out, checker.Contains, name1+".2")
	c.Assert(out, checker.Contains, name1+".3")
	c.Assert(out, checker.Contains, name2+".1")
	c.Assert(out, checker.Contains, name2+".2")
	c.Assert(out, checker.Contains, name2+".3")

	// Name Prefix (no hit)
	out, err = d.Cmd("service", "ps", "noname")
	c.Assert(err, checker.NotNil)
	c.Assert(out, checker.Contains, "no such services: noname")

	out, err = d.Cmd("service", "ps", id1)
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Contains, name1+".1")
	c.Assert(out, checker.Contains, name1+".2")
	c.Assert(out, checker.Contains, name1+".3")
	c.Assert(out, checker.Not(checker.Contains), name2+".1")
	c.Assert(out, checker.Not(checker.Contains), name2+".2")
	c.Assert(out, checker.Not(checker.Contains), name2+".3")

	out, err = d.Cmd("service", "ps", id1, id2)
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Contains, name1+".1")
	c.Assert(out, checker.Contains, name1+".2")
	c.Assert(out, checker.Contains, name1+".3")
	c.Assert(out, checker.Contains, name2+".1")
	c.Assert(out, checker.Contains, name2+".2")
	c.Assert(out, checker.Contains, name2+".3")
}
