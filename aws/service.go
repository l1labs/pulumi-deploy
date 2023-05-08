package aws

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/pulumi/pulumi-aws/sdk/v3/go/aws/cloudwatch"
	"github.com/pulumi/pulumi-aws/sdk/v3/go/aws/ecr"
	"github.com/pulumi/pulumi-aws/sdk/v3/go/aws/ecs"
	"github.com/pulumi/pulumi-docker/sdk/v2/go/docker"
	"github.com/pulumi/pulumi/sdk/v2/go/pulumi"
)

// Service provides around an ECS service.
type Service struct {
	Name   string
	Region string

	Docker  *docker.DockerBuildArgs
	Task    *ecs.TaskDefinitionArgs
	Service *ecs.ServiceArgs

	Ports           []ContainerPortMapping
	LinuxParameters *ContainerLinuxParameters
	MountPoints     []ContainerMountPoint

	Env          pulumi.StringMapInput
	DockerLabels pulumi.StringMapInput

	// Specifies the number of days
	// you want to retain log events in the specified log group.  Possible values are: 1, 3, 5, 7, 14, 30, 60, 90, 120, 150, 180, 365, 400, 545, 731, 1827, and 3653.
	LogRetentionDays int

	Out struct {
		Task    *ecs.TaskDefinition
		Service *ecs.Service
	}
}

// Validate the service configuration.
func (s *Service) Validate() error {
	if s.Name == "" {
		return fmt.Errorf("Missing Name")
	}

	if s.Region == "" {
		return fmt.Errorf("Missing Region")
	}

	if s.Docker == nil {
		return fmt.Errorf("Missing service.Docker args")
	}

	if s.Task == nil {
		return fmt.Errorf("Missing service.Task args")
	}

	if s.Service == nil {
		return fmt.Errorf("Missing service.Service args")
	}

	return nil
}

// Run will run the service confifuration, returning any errors.
func (s *Service) Run(ctx *pulumi.Context) error {
	if err := s.Validate(); err != nil {
		return err
	}

	// Create docker repo
	repo, err := ecr.NewRepository(ctx, s.Name, &ecr.RepositoryArgs{
		Name: pulumi.String(s.Name),
		EncryptionConfigurations: ecr.RepositoryEncryptionConfigurationArray{
			&ecr.RepositoryEncryptionConfigurationArgs{
				EncryptionType: pulumi.String("AES256"),
			},
		},
		ImageTagMutability: pulumi.String("MUTABLE"),
		ImageScanningConfiguration: &ecr.RepositoryImageScanningConfigurationArgs{
			ScanOnPush: pulumi.Bool(true),
		},
	})
	if err != nil {
		return err
	}

	// Get repository credentials
	repoCreds := repo.RegistryId.ApplyStringArray(func(rid string) ([]string, error) {
		creds, err := ecr.GetCredentials(ctx, &ecr.GetCredentialsArgs{
			RegistryId: rid,
		})
		if err != nil {
			return nil, err
		}
		data, err := base64.StdEncoding.DecodeString(creds.AuthorizationToken)
		if err != nil {
			return nil, err
		}
		return strings.Split(string(data), ":"), nil
	})

	repoUser := repoCreds.Index(pulumi.Int(0))
	repoPass := repoCreds.Index(pulumi.Int(1))

	// Create image
	image, err := docker.NewImage(ctx, s.Name, &docker.ImageArgs{
		Build:     *s.Docker,
		ImageName: repo.RepositoryUrl,
		Registry: docker.ImageRegistryArgs{
			Server:   repo.RepositoryUrl,
			Username: repoUser,
			Password: repoPass,
		},
	})
	if err != nil {
		return err
	}

	// Create log group
	logGroup := fmt.Sprintf("/fargate/service/%v", s.Name)
	_, err = cloudwatch.NewLogGroup(ctx, logGroup, &cloudwatch.LogGroupArgs{
		Name: pulumi.String(logGroup),
		Tags: pulumi.StringMap{
			// "Application": pulumi.String("serviceA"),
			// "Environment": pulumi.String("production"),
		},

		RetentionInDays: pulumi.Int(s.LogRetentionDays),
	})
	if err != nil {
		return err
	}

	if s.Env == nil {
		s.Env = pulumi.StringMap{}
	}

	// Create container definition
	containerDef := pulumi.All(image.ImageName, s.Env, s.DockerLabels).ApplyString(
		func(args []interface{}) (string, error) {
			image := args[0].(string)

			envMap, ok := args[1].(map[string]string)
			if !ok {
				return "", fmt.Errorf("Failed to coerce env")
			}

			dockerLabels, ok := args[2].(map[string]string)
			if !ok {
				return "", fmt.Errorf("Failed to coerce dockerLabels")
			}

			env := []ContainerEnvVar{}

			for key, value := range envMap {
				env = append(env, ContainerEnvVar{Name: key, Value: value})
			}

			def := ContainerDefinition{
				Name:            s.Name,
				Image:           image,
				PortMappings:    s.Ports,
				LinuxParameters: s.LinuxParameters,
				MountPoints:     s.MountPoints,
				Environment:     env,
				DockerLabels:    dockerLabels,
				LogConfiguration: &ContainerLogConfig{
					LogDriver:     "awslogs",
					SecretOptions: nil,
					Options: map[string]interface{}{
						"awslogs-group":         logGroup,
						"awslogs-region":        s.Region,
						"awslogs-stream-prefix": "fargate",
					},
				},
			}

			if err := def.Validate(); err != nil {
				return "", err
			}

			return def.String(), nil
		},
	)

	// Setup ECS task & service
	taskName := fmt.Sprintf("%v-task", s.Name)
	appTask, err := ecs.NewTaskDefinition(ctx, taskName, &ecs.TaskDefinitionArgs{
		Family: pulumi.String(taskName),
		Tags: pulumi.StringMap{
			"Name": pulumi.String(taskName),
		},
		Cpu:                     s.Task.Cpu,
		Memory:                  s.Task.Memory,
		NetworkMode:             s.Task.NetworkMode,
		RequiresCompatibilities: s.Task.RequiresCompatibilities,
		ExecutionRoleArn:        s.Task.ExecutionRoleArn,
		ContainerDefinitions:    containerDef,
		Volumes:                 s.Task.Volumes,
		TaskRoleArn:             s.Task.TaskRoleArn,
	})
	if err != nil {
		return err
	}

	s.Out.Task = appTask

	serviceName := fmt.Sprintf("%v-svc", s.Name)
	s.Service.TaskDefinition = appTask.Arn

	service, err := ecs.NewService(ctx, serviceName, s.Service)
	if err != nil {
		return err
	}

	s.Out.Service = service

	return nil
}
