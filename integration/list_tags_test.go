package main

import (
	"github.com/go-check/check"
)

func init() {
	check.Suite(&TagListSuite{})
}

type TagListSuite struct {}

// Simple tag listing
func (s *TagListSuite) TestListSimple(c *check.C) {
	//assertSkopeoSucceeds(c, `.*Repository: docker\.io/library/centos.*`, "list-tags", "docker://docker.io/library/centos")
	//assertSkopeoSucceeds(c, `.*Repository: docker\.io/library/centos.*`, "list-tags", "docker://centos")
	//assertSkopeoSucceeds(c, `.*Repository: docker\.io/library/centos.*`, "list-tags", "docker://docker.io/centos")
	//assertSkopeoFails(c, ".*No tag or digest allowed.*", "", "list-tags", "docker://docker.io/centos:7")
	//ssertSkopeoFails(c, ".*Unsupported transport.*", "", "list-tags", "docker-daemon:docker.io/centos:7")
}
