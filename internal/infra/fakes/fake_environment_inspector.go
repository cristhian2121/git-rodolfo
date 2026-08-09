package fakes

// FakeEnvironmentInspector is an in-memory app.EnvironmentInspector double.
type FakeEnvironmentInspector struct {
	GitVersionValue string
	GitVersionErr   error
	SSHIsInstalled  bool
	Env             map[string]string
}

// NewFakeEnvironmentInspector returns a FakeEnvironmentInspector reporting
// a recent Git version and SSH installed, the common starting point.
func NewFakeEnvironmentInspector() *FakeEnvironmentInspector {
	return &FakeEnvironmentInspector{
		GitVersionValue: "2.43.0",
		SSHIsInstalled:  true,
		Env:             map[string]string{},
	}
}

func (f *FakeEnvironmentInspector) GitVersion() (string, error) {
	return f.GitVersionValue, f.GitVersionErr
}

func (f *FakeEnvironmentInspector) SSHInstalled() bool {
	return f.SSHIsInstalled
}

func (f *FakeEnvironmentInspector) Getenv(key string) string {
	return f.Env[key]
}
