package aws

import (
	"fmt"

	"github.com/pulumi/pulumi-aws/sdk/v3/go/aws/ec2"
	"github.com/pulumi/pulumi-aws/sdk/v3/go/aws/lb"
	"github.com/pulumi/pulumi-aws/sdk/v3/go/aws/s3"

	"github.com/pulumi/pulumi/sdk/v2/go/pulumi"
)

// LoadBalancer is a helper struct for spinning up an ALB
type LoadBalancer struct {
	Name string

	VPC         *VPC
	HTTPS       []*HTTPS
	HealthCheck *lb.TargetGroupHealthCheckArgs
	LogBucket   *s3.Bucket
	LogPrefix   *string

	Out struct {
		SecurityGroup *ec2.SecurityGroup
		LB            *lb.LoadBalancer
		TargetGroup   *lb.TargetGroup
		Listener      *lb.Listener
	}
}

func (l *LoadBalancer) Validate() error {
	if l.Name == "" {
		return fmt.Errorf("Name cannot empty")
	}

	if l.VPC == nil {
		return fmt.Errorf("VPC cannot empty")
	}

	if len(l.HTTPS) == 0 {
		return fmt.Errorf("HTTPS cannot empty")
	}

	if l.HealthCheck == nil {
		l.HealthCheck = &lb.TargetGroupHealthCheckArgs{
			Enabled:            pulumi.Bool(true),
			Path:               pulumi.String("/health"),
			Protocol:           pulumi.String("HTTP"),
			Port:               pulumi.String("80"),
			HealthyThreshold:   pulumi.Int(5),
			UnhealthyThreshold: pulumi.Int(5),
			Timeout:            pulumi.Int(5),
		}
	}

	return nil
}

func (l *LoadBalancer) Run(ctx *pulumi.Context) error {
	// Create a SecurityGroup that permits HTTP ingress and unrestricted egress.
	sgName := fmt.Sprintf("%v-sg", l.Name)
	securityGroup, err := ec2.NewSecurityGroup(ctx, sgName, &ec2.SecurityGroupArgs{
		VpcId: l.VPC.ID(),
		Egress: ec2.SecurityGroupEgressArray{
			ec2.SecurityGroupEgressArgs{
				Protocol:   pulumi.String("-1"),
				FromPort:   pulumi.Int(0),
				ToPort:     pulumi.Int(0),
				CidrBlocks: pulumi.StringArray{pulumi.String("0.0.0.0/0")},
			},
		},
		Ingress: ec2.SecurityGroupIngressArray{
			ec2.SecurityGroupIngressArgs{
				Protocol:   pulumi.String("tcp"),
				FromPort:   pulumi.Int(443),
				ToPort:     pulumi.Int(443),
				CidrBlocks: pulumi.StringArray{pulumi.String("0.0.0.0/0")},
			},
			ec2.SecurityGroupIngressArgs{
				Protocol:   pulumi.String("tcp"),
				FromPort:   pulumi.Int(80),
				ToPort:     pulumi.Int(80),
				CidrBlocks: pulumi.StringArray{pulumi.String("0.0.0.0/0")},
			},
		},
	})
	if err != nil {
		return err
	}
	l.Out.SecurityGroup = securityGroup

	lbName := fmt.Sprintf("%v-lb", l.Name)
	lbArgs := &lb.LoadBalancerArgs{
		Subnets: pulumi.StringArray{
			l.VPC.Out.PublicSubnets[0].ID().ToStringOutput(), l.VPC.Out.PublicSubnets[1].ID().ToStringOutput(),
		},
		Name:                     pulumi.String(lbName),
		LoadBalancerType:         pulumi.String("application"),
		IpAddressType:            pulumi.String("ipv4"),
		SecurityGroups:           pulumi.StringArray{securityGroup.ID().ToStringOutput()},
		DropInvalidHeaderFields:  pulumi.Bool(true),
		EnableDeletionProtection: pulumi.Bool(true),
	}

	if bucket := l.LogBucket; bucket != nil {
		lbArgs.AccessLogs = &lb.LoadBalancerAccessLogsArgs{
			Enabled: pulumi.Bool(true),
			Bucket:  bucket.Bucket,
			Prefix:  pulumi.String(*l.LogPrefix),
		}
	}

	frontEndLoadBalancer, err := lb.NewLoadBalancer(ctx, lbName, lbArgs)
	if err != nil {
		return err
	}
	l.Out.LB = frontEndLoadBalancer

	tgName := fmt.Sprintf("%v-tg", l.Name)
	frontEndTargetGroup, err := lb.NewTargetGroup(ctx, tgName, &lb.TargetGroupArgs{
		Name:                pulumi.String(tgName),
		Port:                pulumi.Int(80),
		Protocol:            pulumi.String("HTTP"),
		VpcId:               l.VPC.ID(),
		TargetType:          pulumi.String("ip"),
		DeregistrationDelay: pulumi.Int(30),
		HealthCheck:         l.HealthCheck,
	})
	if err != nil {
		return err
	}
	l.Out.TargetGroup = frontEndTargetGroup

	listenerName := fmt.Sprintf("%v-listener", l.Name)
	frontEndListener, err := lb.NewListener(ctx, listenerName, &lb.ListenerArgs{
		LoadBalancerArn: frontEndLoadBalancer.Arn,
		Port:            pulumi.Int(443),
		Protocol:        pulumi.String("HTTPS"),
		SslPolicy:       pulumi.String("ELBSecurityPolicy-FS-1-2-Res-2020-10"),
		CertificateArn:  l.HTTPS[0].Out.Cert.Arn,
		DefaultActions: lb.ListenerDefaultActionArray{
			&lb.ListenerDefaultActionArgs{
				Type:           pulumi.String("forward"),
				TargetGroupArn: frontEndTargetGroup.Arn,
			},
		},
	})
	if err != nil {
		return err
	}
	l.Out.Listener = frontEndListener

	if len(l.HTTPS) > 1 {
		for i := 1; i < len(l.HTTPS); i++ {
			name := fmt.Sprintf("%v-listener-cert-%d", l.Name, i)
			_, err = lb.NewListenerCertificate(ctx, name, &lb.ListenerCertificateArgs{
				ListenerArn:    frontEndListener.Arn,
				CertificateArn: l.HTTPS[i].Out.Cert.Arn,
			})

			if err != nil {
				return err
			}
		}
	}

	return nil
}
