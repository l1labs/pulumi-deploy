package aws

import (
	"fmt"
	"strings"

	"github.com/pulumi/pulumi-aws/sdk/v5/go/aws/cloudwatch"
	"github.com/pulumi/pulumi-aws/sdk/v5/go/aws/ecs"
	"github.com/pulumi/pulumi-docker/sdk/v3/go/docker"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
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

	SidecarContainers pulumi.StringArrayInput

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
		return fmt.Errorf("missing Service.Name")
	}

	if s.Region == "" {
		return fmt.Errorf("missing Service.Region")
	}

	if s.Docker == nil {
		return fmt.Errorf("missing Service.Docker args")
	}

	if s.Task == nil {
		return fmt.Errorf("missing Service.Task args")
	}

	if s.Service == nil {
		return fmt.Errorf("missing Service.Service args")
	}

	return nil
}

// Run will run the service confifuration, returning any errors.
func (s *Service) Run(ctx *pulumi.Context) error {
	if err := s.Validate(); err != nil {
		return err
	}

	d := &Docker{
		Name:   s.Name,
		Docker: s.Docker,
	}

	if err := d.Run(ctx); err != nil {
		return err
	}

	// Create log group
	logConfiguration, err := ServiceLogConfiguration(ctx, s.Name, s.Region, s.LogRetentionDays)
	if err != nil {
		return err
	}

	if s.Env == nil {
		s.Env = pulumi.StringMap{}
	}

	// Create container definition
	containerDef := pulumi.All(d.Out.Image.ImageName, s.Env, s.DockerLabels, s.SidecarContainers, logConfiguration).ApplyT(
		func(args []interface{}) (string, error) {
			image := args[0].(string)

			envMap, ok := args[1].(map[string]string)
			if !ok {
				return "", fmt.Errorf("failed to coerce env")
			}

			dockerLabels, ok := args[2].(map[string]string)
			if !ok {
				return "", fmt.Errorf("failed to coerce dockerLabels")
			}

			sidecarContainers, ok := args[3].([]string)
			if !ok {
				return "", fmt.Errorf("Failed to coerce sidecar containers")
			}

			logConfig, ok := args[4].(*ContainerLogConfig)
			if !ok {
				return "", fmt.Errorf("Failed to coerce container log config")
			}

			env := []ContainerEnvVar{}

			for key, value := range envMap {
				env = append(env, ContainerEnvVar{Name: key, Value: value})
			}

			def := ContainerDefinition{
				Name:             s.Name,
				Image:            image,
				PortMappings:     s.Ports,
				LinuxParameters:  s.LinuxParameters,
				MountPoints:      s.MountPoints,
				Environment:      env,
				DockerLabels:     dockerLabels,
				LogConfiguration: logConfig,
			}

			if err := def.Validate(); err != nil {
				return "", err
			}

			containers := []string{
				def.String(),
			}
			containers = append(containers, sidecarContainers...)

			return "[" + strings.Join(containers, ",") + "]", nil
		},
	).(pulumi.StringInput)

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

func ServiceLogConfiguration(ctx *pulumi.Context, name, region string, logRetentionDays int) (*ContainerLogConfig, error) {
	logGroup := fmt.Sprintf("/fargate/service/%v", name)
	_, err := cloudwatch.NewLogGroup(ctx, logGroup, &cloudwatch.LogGroupArgs{
		Name:            pulumi.String(logGroup),
		Tags:            pulumi.StringMap{},
		RetentionInDays: pulumi.Int(logRetentionDays),
	})
	if err != nil {
		return nil, err
	}

	return &ContainerLogConfig{
		LogDriver:     "awslogs",
		SecretOptions: nil,
		Options: map[string]interface{}{
			"awslogs-group":         logGroup,
			"awslogs-region":        region,
			"awslogs-stream-prefix": "fargate",
		},
	}, nil
}
