//go:build test

// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	glog "gopkg.in/op/go-logging.v1"
)

// In production main func will take care of setting the log yq-lib log level.
// In test we count on this to set the default log level for all tests.
// We do this because yqlib log is very chatty even if it is sometimes very useful.
// Of course in an individual test one could change the level and reset to the default
// via a defer call.
func init() {
	glog.SetLevel(glog.WARNING, "yq-lib")
}
