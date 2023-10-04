package aws

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/pulumi/pulumi-aws/sdk/v5/go/aws/ecr"
	"github.com/pulumi/pulumi-docker/sdk/v3/go/docker"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type Docker struct {
	Name   string
	Docker *docker.DockerBuildArgs

	Out struct {
		Repo  *ecr.Repository
		Image *docker.Image
	}
}

func (d *Docker) Validate() error {
	if d.Name == "" {
		return fmt.Errorf("Missing Name")
	}

	if d.Docker == nil {
		return fmt.Errorf("Missing docker.Docker args")
	}

	return nil
}

func (d *Docker) Run(ctx *pulumi.Context, opts ...pulumi.ResourceOption) error {
	if err := d.Validate(); err != nil {
		return err
	}

	// Create docker repo
	repo, err := ecr.NewRepository(ctx, d.Name, &ecr.RepositoryArgs{
		Name: pulumi.String(d.Name),
		EncryptionConfigurations: ecr.RepositoryEncryptionConfigurationArray{
			&ecr.RepositoryEncryptionConfigurationArgs{
				EncryptionType: pulumi.String("AES256"),
			},
		},
		ImageTagMutability: pulumi.String("MUTABLE"),
		ImageScanningConfiguration: &ecr.RepositoryImageScanningConfigurationArgs{
			ScanOnPush: pulumi.Bool(true),
		},
	}, opts...)
	if err != nil {
		return err
	}

	d.Out.Repo = repo

	// Get repository credentials
	repoCreds := repo.RegistryId.ApplyT(func(rid string) ([]string, error) {
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
	}).(pulumi.StringArrayOutput)

	repoUser := repoCreds.Index(pulumi.Int(0))
	repoPass := repoCreds.Index(pulumi.Int(1))

	// Create image
	image, err := docker.NewImage(ctx, d.Name, &docker.ImageArgs{
		Build:     *d.Docker,
		ImageName: repo.RepositoryUrl,
		Registry: docker.ImageRegistryArgs{
			Server:   repo.RepositoryUrl,
			Username: repoUser,
			Password: repoPass,
		},
	}, opts...)
	if err != nil {
		return err
	}

	d.Out.Image = image

	return nil
}
