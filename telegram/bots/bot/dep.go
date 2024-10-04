package bot

//go:generate mockgen -source dep.go -destination ./dep_mock_test.go -package ${GOPACKAGE}

type Logger interface {
	Panicf(template string, args ...any)
	Errorf(template string, args ...any)
	Warnf(template string, args ...any)
	Infof(template string, args ...any)
	Debugf(template string, args ...any)
}
