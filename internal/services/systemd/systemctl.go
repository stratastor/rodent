package systemd

import "fmt"

type ServiceStatus struct {
	Name    string `json:"name"`
	Service string `json:"service"`
	Status  string `json:"status"`
	Health  string `json:"health"`
	State   string `json:"state"`
}

func (s ServiceStatus) String() string {
	return fmt.Sprintf(
		"%s (%s) is %s [%s]",
		s.Name,
		s.Service,
		s.State,
		s.Status,
	)
}

func (s ServiceStatus) InstanceGist() string {
	return s.String()
}

func (s ServiceStatus) InstanceName() string {
	return s.Name
}

func (s ServiceStatus) InstanceService() string {
	return s.Service
}

func (s ServiceStatus) InstanceStatus() string {
	return s.Status
}

func (s ServiceStatus) InstanceHealth() string {
	return s.Health
}

func (s ServiceStatus) InstanceState() string {
	return s.State
}
