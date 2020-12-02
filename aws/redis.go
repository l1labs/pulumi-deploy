package aws

import (
	"fmt"

	"github.com/pulumi/pulumi-aws/sdk/v3/go/aws/ec2"
	"github.com/pulumi/pulumi-aws/sdk/v3/go/aws/elasticache"
	"github.com/pulumi/pulumi/sdk/v2/go/pulumi"
)

// Redis contains everything needed to spin up a secure Redis instance.
type Redis struct {
	Name   string
	Subnet *ec2.Subnet
	Args   *elasticache.ClusterArgs

	Out struct {
		Cache *elasticache.Cluster
	}
}

func (r *Redis) Validate() error {
	if r.Name == "" {
		return fmt.Errorf("Name cannot empty")
	}

	if r.Args == nil {
		return fmt.Errorf("Missing Redis.Args")
	}

	if r.Subnet == nil {
		return fmt.Errorf("Subnet cannot empty")
	}

	return nil
}

func (r *Redis) Run(ctx *pulumi.Context) error {
	if err := r.Validate(); err != nil {
		return err
	}

	redisSubnetName := fmt.Sprintf("%v-subnet", r.Name)
	redisSubnet, err := elasticache.NewSubnetGroup(ctx, redisSubnetName, &elasticache.SubnetGroupArgs{
		Name: pulumi.String(redisSubnetName),
		SubnetIds: pulumi.StringArray{
			r.Subnet.ID(),
		},
	}, pulumi.DependsOn([]pulumi.Resource{r.Subnet}))
	if err != nil {
		return err
	}

	r.Args.SubnetGroupName = redisSubnet.Name

	/*
		&elasticache.ClusterArgs{
			Engine:             pulumi.String("redis"),
			EngineVersion:      pulumi.String("3.2.10"),
			NodeType:           pulumi.String("cache.t3.micro"),
			NumCacheNodes:      pulumi.Int(1),
			ParameterGroupName: pulumi.String("default.redis3.2"),
			Port:               pulumi.Int(6379),
			SubnetGroupName:    redisSubnet.Name,
		}
	*/
	cache, err := elasticache.NewCluster(ctx, r.Name, r.Args,
		pulumi.DependsOn([]pulumi.Resource{redisSubnet}))
	if err != nil {
		return err
	}

	r.Out.Cache = cache

	return nil
}
