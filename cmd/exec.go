package cmd

import (
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// Network is the type task network mode setting
type Network struct {
	NetworkMode   string   `json:"NetworkMode"`
	IPv4Addresses []string `json:"IPv4Addresses"`
}

// PortMapping is the type task port mapping setting
type PortMapping struct {
	ContainerPort int64  `json:"ContainerPort"`
	HostPort      int64  `json:"HostPort"`
	BindIP        string `json:"BindIp"`
	Protocol      string `json:"Protocol"`
}

// ContainerMeta is the struct for ecs task meta data
type ContainerMeta struct {
	// https://docs.aws.amazon.com/ja_jp/AmazonECS/latest/developerguide/container-metadata.html
	Cluster              string         `json:"Cluster"`
	ContainerInstanceARN string         `json:"ContainerInstanceARN"`
	TaskARN              string         `json:"TaskARN"`
	ContainerName        string         `json:"ContainerName"`
	ContainerId          *string        `json:"ContainerID"`
	DockerContainerName  *string        `json:"DockerContainerName"`
	ImageID              *string        `json:"ImageID"`
	ImageName            *string        `json:"ImageName"`
	PortMappings         []*PortMapping `json:"PortMappings"`
	Networks             []*Network     `json:"Networks"`
	MetadataFileStatus   *string        `json:"MetadataFileStatus"`
}

func readMetaFile(metadataFilePath string) (*ContainerMeta, error) {
	var containerMeta ContainerMeta

	bytes, err := ioutil.ReadFile(metadataFilePath)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to open ECS_CONTAINER_METADATA_FILE")
	}
	if err := json.Unmarshal(bytes, &containerMeta); err != nil {
		return nil, errors.Wrap(err, "Failed to parse ECS_CONTAINER_METADATA_FILE")
	}

	return &containerMeta, nil
}

func execRun(cmd *cobra.Command, args []string) error {
	dashIx := cmd.ArgsLenAtDash()
	command, commandArgs := args[dashIx], args[dashIx+1:]

	env := environ(os.Environ())

	// Read $ECS_CONTAINER_METADATA_FILE
	metadataFilePath, res := os.LookupEnv("ECS_CONTAINER_METADATA_FILE")
	if !res {
		return errors.New("Failed to find ECS_CONTAINER_METADATA_FILE environment")
	}

	// Read meta file until state is ready
	tryCount := 10
	for i := 0; i < tryCount; i++ {
		// parse and export port mappings
		containerMeta, err := readMetaFile(metadataFilePath)
		if err != nil {
			return err
		}

		if containerMeta.MetadataFileStatus == nil || *containerMeta.MetadataFileStatus != "READY" {
			if i == (tryCount - 1) {
				return errors.New("Failed to read ECS_CONTAINER_METADATA_FILE file, because it is not ready")
			}
			time.Sleep(1 * time.Second)
			continue
		}

		setEnvironments(containerMeta, env)
		break // if file reading succeeded, break the loop
	}


	return exec(command, commandArgs, env)
}

func setEnvironments(containerMeta *ContainerMeta, env environ) {
	// 1. set env keys into envs
	envs := map[string]string{}
	putEnvKeyValue := func (key, value string) {
		_, exist := envs[key]
		if exist {
			fmt.Fprintf(os.Stderr, "warning: overwriting environment variable %s\n", key)
		}
		envs[key] = value
	}

	// container id mapping
	putEnvKeyValue("CONTAINER_ID", *containerMeta.ContainerId)

	// port mapping
	for _, portMapping := range containerMeta.PortMappings {
		protocol := strings.ToUpper(portMapping.Protocol)
		containerPort := fmt.Sprintf("%d", portMapping.ContainerPort)
		value := fmt.Sprintf("%d", portMapping.HostPort)
		key := fmt.Sprintf("PORT_%s_%s", protocol, containerPort)
		putEnvKeyValue(key, value)
	}

	// 2. set environment from envs
	for key, value := range envs {
		env.Set(key, value)
		if verbose {
			fmt.Fprintf(os.Stdout, "info: With environment %s=%s\n", key, value)
		}
	}
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
