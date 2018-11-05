package cmd

import (
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	"io/ioutil"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

type Network struct {
	NetworkMode   string   `json:"NetworkMode"`
	IPv4Addresses []string `json:"IPv4Addresses"`
}

type PortMapping struct {
	ContainerPort int64  `json:"ContainerPort"`
	HostPort      int64  `json:"HostPort"`
	BindIp        string `json:"BindIp"`
	Protocol      string `json:"Protocol"`
}

type ContainerMeta struct {
	Cluster              string         `json:"Cluster"`
	ContainerInstanceARN string         `json:"ContainerInstanceARN"`
	TaskARN              string         `json:"TaskARN"`
	ContainerName        string         `json:"ContainerName"`
	DockerContainerName  *string        `json:"DockerContainerName"`
	ImageID              *string        `json:"ImageID"`
	ImageName            *string        `json:"ImageName"`
	PortMappings         []*PortMapping `json:"PortMappings"`
	Networks             []*Network     `json:"Networks"`
	MetadataFileStatus   *string        `json:"MetadataFileStatus"`
}

func execRun(cmd *cobra.Command, args []string) error {
	dashIx := cmd.ArgsLenAtDash()
	command, commandArgs := args[dashIx], args[dashIx+1:]

	env := environ(os.Environ())
	envVarKeys := make([]string, 0)

	// Read $ECS_CONTAINER_METADATA_FILE
	metadataFilePath, res := os.LookupEnv("ECS_CONTAINER_METADATA_FILE")
	if !res {
		return errors.New("Failed to find ECS_CONTAINER_METADATA_FILE")
	}
	bytes, err := ioutil.ReadFile(metadataFilePath)
	if err != nil {
		return errors.Wrap(err, "Failed to open ECS_CONTAINER_METADATA_FILE")
	}
	var containerMeta ContainerMeta
	if err := json.Unmarshal(bytes, &containerMeta); err != nil {
		return errors.Wrap(err, "Failed to parse ECS_CONTAINER_METADATA_FILE")
	}

	// TODO: Ready になるまでの時間があるため、ループにする必要あり

	// parse and export port mappings
	for _, portMapping := range containerMeta.PortMappings {

		protocol := strings.ToUpper(portMapping.Protocol)
		containerPort := fmt.Sprintf("%d", portMapping.ContainerPort)
		hostPort := fmt.Sprintf("%d", portMapping.HostPort)

		envVarKey := fmt.Sprintf("PORT_%s_%s", protocol, containerPort)
		envVarKeys = append(envVarKeys, envVarKey)

		if env.IsSet(envVarKey) {
			fmt.Fprintf(os.Stderr, "warning: overwriting environment variable %s\n", envVarKey)
		}
		env.Set(envVarKey, hostPort)
	}

	if verbose {
		fmt.Fprintf(os.Stdout, "info: With environment %s\n", strings.Join(envVarKeys, ","))
	}

	return exec(command, commandArgs, env)
}

// environ is a slice of strings representing the environment, in the form "key=value".
type environ []string

// Unset an environment variable by key
func (e *environ) Unset(key string) {
	for i := range *e {
		if strings.HasPrefix((*e)[i], key+"=") {
			(*e)[i] = (*e)[len(*e)-1]
			*e = (*e)[:len(*e)-1]
			break
		}
	}
}

// IsSet returns whether or not a key is currently set in the environ
func (e *environ) IsSet(key string) bool {
	for i := range *e {
		if strings.HasPrefix((*e)[i], key+"=") {
			return true
		}
	}
	return false
}

// Set adds an environment variable, replacing any existing ones of the same key
func (e *environ) Set(key, val string) {
	e.Unset(key)
	*e = append(*e, key+"="+val)
}