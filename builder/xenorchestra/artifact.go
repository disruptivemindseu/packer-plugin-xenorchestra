package xenorchestra

type Artifact struct {
	StateData map[string]interface{}
}

func (a *Artifact) BuilderId() string {
	return BuilderId
}

func (a *Artifact) Files() []string {
	return []string{}
}

func (a *Artifact) Id() string {
	return ""
}

func (a *Artifact) String() string {
	return "Xen Orchestra artifact"
}

func (a *Artifact) State(name string) interface{} {
	return a.StateData[name]
}

func (a *Artifact) Destroy() error {
	return nil
}
