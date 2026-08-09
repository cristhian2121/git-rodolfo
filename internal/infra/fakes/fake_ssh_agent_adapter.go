package fakes

// FakeSSHAgentAdapter is an in-memory app.SSHAgentAdapter double.
type FakeSSHAgentAdapter struct {
	Running    bool
	LoadedKeys map[string]bool
	AddErr     error
}

// NewFakeSSHAgentAdapter returns a FakeSSHAgentAdapter with the agent
// running and no keys loaded, the common starting point for tests.
func NewFakeSSHAgentAdapter() *FakeSSHAgentAdapter {
	return &FakeSSHAgentAdapter{Running: true, LoadedKeys: map[string]bool{}}
}

func (f *FakeSSHAgentAdapter) IsRunning() bool {
	return f.Running
}

func (f *FakeSSHAgentAdapter) IsKeyLoaded(keyPath string) (bool, error) {
	return f.LoadedKeys[keyPath], nil
}

func (f *FakeSSHAgentAdapter) AddKey(keyPath string, useKeychain bool) error {
	if f.AddErr != nil {
		return f.AddErr
	}
	f.LoadedKeys[keyPath] = true
	return nil
}
