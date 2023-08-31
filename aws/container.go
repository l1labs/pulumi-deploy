package aws

import (
	"encoding/json"
	"fmt"
)

type ContainerDefinition struct {
	Command          []string                  `json:"command,omitempty"`
	Name             string                    `json:"name"`
	Image            string                    `json:"image"`
	PortMappings     []ContainerPortMapping    `json:"portMappings"`
	Environment      []ContainerEnvVar         `json:"environment"`
	LogConfiguration *ContainerLogConfig       `json:"logConfiguration"`
	DockerLabels     map[string]string         `json:"dockerLabels"`
	LinuxParameters  *ContainerLinuxParameters `json:"linuxParameters,omitempty"`
	MountPoints      []ContainerMountPoint     `json:"mountPoints,omitempty"`
}

func (d *ContainerDefinition) Validate() error {
	if d.Name == "" {
		return fmt.Errorf("Missing Name")
	}

	if d.Image == "" {
		return fmt.Errorf("Missing Image")
	}

	if d.PortMappings == nil {
		d.PortMappings = []ContainerPortMapping{}
	}

	if d.Environment == nil {
		d.Environment = []ContainerEnvVar{}
	}

	if d.LogConfiguration == nil {
		return fmt.Errorf("Missing LogConfiguration")
	}

	return nil
}

func (d *ContainerDefinition) String() string {
	data, _ := json.Marshal(d)

	return string(data)
}

type ContainerLinuxParameters struct {
	Capabilities ContainerLinuxCapabilities `json:"capabilities"`
}

type ContainerLinuxCapabilities struct {
	Add  []string `json:"add"`
	Drop []string `json:"drop"`
}

type ContainerEnvVar struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}
type ContainerPortMapping struct {
	ContainerPort int    `json:"containerPort"`
	HostPort      int    `json:"hostPort"`
	Protocol      string `json:"protocol"`
}

type ContainerLogConfig struct {
	LogDriver     string                 `json:"logDriver"`
	SecretOptions interface{}            `json:"secretOptions"`
	Options       map[string]interface{} `json:"options"`
}

type ContainerMountPoint struct {
	ContainerPath string `json:"containerPath"`
	ReadOnly      bool   `json:"readOnly"`
	SourceVolume  string `json:"sourceVolume"`
}
