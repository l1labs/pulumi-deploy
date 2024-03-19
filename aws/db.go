package aws

import (
	"fmt"

	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/rds"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type Postgres struct {
	Name    string
	Args    *rds.InstanceArgs
	VPC     *VPC
	Replica bool

	Out struct {
		DB *rds.Instance
	}
}

func (d *Postgres) Validate() error {
	if d.Name == "" {
		return fmt.Errorf("missing Postgres.Name")
	}

	if d.Args == nil {
		return fmt.Errorf("missing Postgres.Args")
	}

	if d.VPC == nil {
		return fmt.Errorf("missing Postgres.VPC")
	}

	return nil
}

func (d *Postgres) Run(ctx *pulumi.Context, opts ...pulumi.ResourceOption) error {
	if err := d.Validate(); err != nil {
		return err
	}

	if opts == nil {
		opts = []pulumi.ResourceOption{}
	}

	if d.Args.ReplicateSourceDb == nil {
		dbSubnetName := fmt.Sprintf("%v-db-subnet", d.Name)
		dbSubnet, err := rds.NewSubnetGroup(ctx, dbSubnetName, &rds.SubnetGroupArgs{
			SubnetIds: pulumi.StringArray{
				d.VPC.Out.PublicSubnets[0].ID().ToStringOutput(), d.VPC.Out.PublicSubnets[1].ID().ToStringOutput(),
				d.VPC.Out.PrivateSubnets[0].ID().ToStringOutput(), d.VPC.Out.PrivateSubnets[1].ID().ToStringOutput(),
			},
			Tags: pulumi.StringMap{
				"Name": pulumi.String(dbSubnetName),
			},
		}, pulumi.DependsOn([]pulumi.Resource{
			d.VPC.Out.PublicSubnets[0], d.VPC.Out.PublicSubnets[1],
			d.VPC.Out.PrivateSubnets[0], d.VPC.Out.PrivateSubnets[1],
		}),
		)
		if err != nil {
			return err
		}

		d.Args.DbSubnetGroupName = dbSubnet.Name
		opts = append(opts, pulumi.DependsOn([]pulumi.Resource{dbSubnet}))
	}

	if !d.Replica {
		d.Args.DbName = pulumi.String(d.Name)
	}

	// dbArgs := rds.InstanceArgs{
	// 	AllocatedStorage:    pulumi.Int(20),
	// 	MaxAllocatedStorage: pulumi.Int(100),
	// 	Engine:              pulumi.String("postgres"),
	// 	EngineVersion:       pulumi.String("12.4"),
	// 	InstanceClass:       pulumi.String("db.t2.micro"),
	// 	Name:                pulumi.String(d.Name),
	// 	MultiAz:             pulumi.Bool(false),
	// 	ParameterGroupName:  pulumi.String("default.postgres12"),
	// 	PubliclyAccessible:  pulumi.Bool(false),
	// 	Password:            d.PasswordSecret,
	// 	StorageType:         pulumi.String("gp2"),
	// 	Username:            pulumi.String(d.Username),
	// 	DbSubnetGroupName:   dbSubnet.Name,
	// 	SkipFinalSnapshot:   pulumi.Bool(d.SkipFinalSnapshot),
	// }

	database, err := rds.NewInstance(ctx, d.Name, d.Args, opts...)
	if err != nil {
		return err
	}

	d.Out.DB = database

	return nil
}
