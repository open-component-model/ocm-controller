package controllers

import "gopkg.in/op/go-logging.v1"

type devNullLogger struct{}

func (d *devNullLogger) Log(_ logging.Level, _ int, _ *logging.Record) error {
	return nil
}

func (d *devNullLogger) GetLevel(_ string) logging.Level {
	return logging.DEBUG
}

func (d *devNullLogger) SetLevel(_ logging.Level, _ string) {}

func (d *devNullLogger) IsEnabledFor(_ logging.Level, _ string) bool {
	return false
}
