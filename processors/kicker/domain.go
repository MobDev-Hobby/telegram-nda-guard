package kicker

import (
	"go.uber.org/zap"
)

type Domain struct {
	log        Logger
	restrictor UserRestrictor
	// cleanMessages, keepBanned and cleanUnknown are the process-wide defaults.
	// They are used when an AccessReport does not carry per-channel
	// CleanOptions. Per-channel CleanOptions take precedence.
	cleanMessages bool
	keepBanned    bool
	cleanUnknown  bool
}

func New(
	restrictor UserRestrictor,
	opts ...Option,
) *Domain {

	d := &Domain{
		log:           Logger(zap.NewNop().Sugar()),
		restrictor:    restrictor,
		keepBanned:    false,
		cleanMessages: true,
		cleanUnknown:  false,
	}

	for _, opt := range opts {
		opt(d)
	}

	return d
}
