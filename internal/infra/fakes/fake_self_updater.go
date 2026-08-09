package fakes

// FakeSelfUpdater is an in-memory app.SelfUpdater double.
type FakeSelfUpdater struct {
	Replaced [][]byte
	Err      error
}

func (f *FakeSelfUpdater) Replace(newBinary []byte) error {
	if f.Err != nil {
		return f.Err
	}
	f.Replaced = append(f.Replaced, newBinary)
	return nil
}
