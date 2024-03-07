package aws

import (
	"fmt"

	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/ec2"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type VPC struct {
	Name      string
	CidrBlock string
	Region    string

	PublicSubnetCidrBlocks  []string
	PrivateSubnetCidrBlocks []string

	Out struct {
		VPC            *ec2.Vpc
		PublicSubnets  []*ec2.Subnet
		PrivateSubnets []*ec2.Subnet
	}
}

func (v *VPC) ID() pulumi.IDOutput {
	return v.Out.VPC.ID()
}

func (v *VPC) Validate() error {
	if v.Name == "" {
		return fmt.Errorf("missing VPC.Name")
	}

	if v.CidrBlock == "" {
		return fmt.Errorf("missing VPC.CidrBlock")
	}

	if v.PublicSubnetCidrBlocks == nil {
		return fmt.Errorf("missing VPC.PublicSubnetCidrBLocks")
	}

	if len(v.PublicSubnetCidrBlocks) < 2 {
		return fmt.Errorf("VPC.PublicSubnetCidrBLocks must have at least 2 CIDR blocks")
	}

	if v.PrivateSubnetCidrBlocks == nil {
		return fmt.Errorf("missing VPC.PrivateSubnetCidrBlocks")
	}

	if len(v.PrivateSubnetCidrBlocks) < 2 {
		return fmt.Errorf("VPC.PrivateSubnetCidrBlocks must have at least 2 CIDR blocks")
	}

	if v.Region == "" {
		return fmt.Errorf("missing VPC.Region")
	}

	return nil
}

func (v *VPC) Run(ctx *pulumi.Context) error {
	if err := v.Validate(); err != nil {
		return err
	}

	vpcName := fmt.Sprintf("%v-vpc", v.Name)
	vpcArgs := &ec2.VpcArgs{
		EnableDnsHostnames: pulumi.BoolPtr(true),
		CidrBlock:          pulumi.String(v.CidrBlock),
		Tags: pulumi.StringMap{
			"Name": pulumi.String(vpcName),
		},
	}

	vpc, err := ec2.NewVpc(ctx, fmt.Sprintf("%v-vpc", v.Name), vpcArgs)
	if err != nil {
		return err
	}
	v.Out.VPC = vpc

	ctx.Export("VPC-ID", vpc.ID())

	// Create public sub
	publicSubnetName := fmt.Sprintf("%v-public-subnet-1", v.Name)
	publicSubnet1, err := ec2.NewSubnet(ctx, publicSubnetName, &ec2.SubnetArgs{
		Tags: pulumi.StringMap{
			"Name": pulumi.String(publicSubnetName),
		},
		VpcId:            vpc.ID(),
		CidrBlock:        pulumi.String(v.PublicSubnetCidrBlocks[0]),
		AvailabilityZone: pulumi.StringPtr(fmt.Sprintf("%va", v.Region)),
	})
	if err != nil {
		return err
	}

	publicSubnet2Name := fmt.Sprintf("%v-public-subnet-2", v.Name)
	publicSubnet2, err := ec2.NewSubnet(ctx, publicSubnet2Name, &ec2.SubnetArgs{
		Tags: pulumi.StringMap{
			"Name": pulumi.String(publicSubnet2Name),
		},
		VpcId:            vpc.ID(),
		CidrBlock:        pulumi.String(v.PublicSubnetCidrBlocks[1]),
		AvailabilityZone: pulumi.StringPtr(fmt.Sprintf("%vc", v.Region)),
	})
	if err != nil {
		return err
	}

	// Create private subnets
	privateSubnet1Name := fmt.Sprintf("%v-private-subnet-1", v.Name)
	privateSubnet1, err := ec2.NewSubnet(ctx, privateSubnet1Name, &ec2.SubnetArgs{
		Tags: pulumi.StringMap{
			"Name": pulumi.String(privateSubnet1Name),
		},
		VpcId:            vpc.ID(),
		CidrBlock:        pulumi.String(v.PrivateSubnetCidrBlocks[0]),
		AvailabilityZone: pulumi.StringPtr(fmt.Sprintf("%va", v.Region)),
	})
	if err != nil {
		return err
	}

	privateSubnet2Name := fmt.Sprintf("%v-private-subnet-2", v.Name)
	privateSubnet2, err := ec2.NewSubnet(ctx, privateSubnet2Name, &ec2.SubnetArgs{
		Tags: pulumi.StringMap{
			"Name": pulumi.String(privateSubnet2Name),
		},
		VpcId:            vpc.ID(),
		CidrBlock:        pulumi.String(v.PrivateSubnetCidrBlocks[1]),
		AvailabilityZone: pulumi.StringPtr(fmt.Sprintf("%vc", v.Region)),
	})
	if err != nil {
		return err
	}

	v.Out.PublicSubnets = []*ec2.Subnet{publicSubnet1, publicSubnet2}
	v.Out.PrivateSubnets = []*ec2.Subnet{privateSubnet1, privateSubnet2}

	igName := fmt.Sprintf("%v-internet-gateway", v.Name)
	internetGateway, err := ec2.NewInternetGateway(ctx, igName, &ec2.InternetGatewayArgs{
		Tags: pulumi.StringMap{
			"Name": pulumi.String(igName),
		},
		VpcId: vpc.ID(),
	}, pulumi.DependsOn([]pulumi.Resource{vpc}))
	if err != nil {
		return err
	}

	ctx.Export("IGW-ID", internetGateway.ID())

	eipName := fmt.Sprintf("%v-nat-gateway-ip", v.Name)
	elasticIPAllocation, err := ec2.NewEip(ctx, eipName, &ec2.EipArgs{
		Tags: pulumi.StringMap{
			"Name": pulumi.String(eipName),
		},
		Vpc: pulumi.Bool(true),
	})
	if err != nil {
		return err
	}

	natName := fmt.Sprintf("%v-nat-gateway", v.Name)
	natGateway, err := ec2.NewNatGateway(ctx, natName, &ec2.NatGatewayArgs{
		Tags: pulumi.StringMap{
			"Name": pulumi.String(natName),
		},
		AllocationId: elasticIPAllocation.ID(),
		SubnetId:     v.Out.PublicSubnets[0].ID(),
	}, pulumi.DependsOn([]pulumi.Resource{v.Out.PublicSubnets[0], elasticIPAllocation}))
	if err != nil {
		return err
	}

	ctx.Export("NAT-GATEWAY-ID", natGateway.ID())

	pubRouteTableName := fmt.Sprintf("%v-public-route-table", v.Name)
	publicSubnetRouteTable, err := ec2.NewRouteTable(ctx, pubRouteTableName, &ec2.RouteTableArgs{
		VpcId: vpc.ID(),
		Tags: pulumi.StringMap{
			"Name": pulumi.String(pubRouteTableName),
		},
		Routes: ec2.RouteTableRouteArray{
			&ec2.RouteTableRouteArgs{
				CidrBlock: pulumi.String("0.0.0.0/0"),
				GatewayId: internetGateway.ID(),
			},
		},
	})

	if err != nil {
		return err
	}

	privateRouteTableName := fmt.Sprintf("%v-private-route-table", v.Name)
	privateSubnetRouteTable, err := ec2.NewRouteTable(ctx, privateRouteTableName, &ec2.RouteTableArgs{
		VpcId: vpc.ID(),
		Tags: pulumi.StringMap{
			"Name": pulumi.String(privateRouteTableName),
		},
		Routes: ec2.RouteTableRouteArray{
			&ec2.RouteTableRouteArgs{
				CidrBlock:    pulumi.String("0.0.0.0/0"),
				NatGatewayId: natGateway.ID(),
			},
		},
	})

	if err != nil {
		return err
	}

	_, err = ec2.NewRouteTableAssociation(ctx, fmt.Sprintf("%v-public-subnet-1-rt-assoc", v.Name), &ec2.RouteTableAssociationArgs{
		SubnetId:     v.Out.PublicSubnets[0].ID(),
		RouteTableId: publicSubnetRouteTable.ID(),
	})
	if err != nil {
		return err
	}

	_, err = ec2.NewRouteTableAssociation(ctx, fmt.Sprintf("%v-public-subnet-2-rt-assoc", v.Name), &ec2.RouteTableAssociationArgs{
		SubnetId:     v.Out.PublicSubnets[1].ID(),
		RouteTableId: publicSubnetRouteTable.ID(),
	})
	if err != nil {
		return err
	}

	_, err = ec2.NewRouteTableAssociation(ctx, fmt.Sprintf("%v-private-subnet-1-rt-assoc", v.Name), &ec2.RouteTableAssociationArgs{
		SubnetId:     privateSubnet1.ID(),
		RouteTableId: privateSubnetRouteTable.ID(),
	})
	if err != nil {
		return err
	}

	_, err = ec2.NewRouteTableAssociation(ctx, fmt.Sprintf("%v-private-subnet-2-rt-assoc", v.Name), &ec2.RouteTableAssociationArgs{
		SubnetId:     privateSubnet2.ID(),
		RouteTableId: privateSubnetRouteTable.ID(),
	})
	if err != nil {
		return err
	}

	return nil
}
