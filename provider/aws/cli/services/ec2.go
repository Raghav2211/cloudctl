package services

import (
	"cloudctl/global"
	"cloudctl/provider/aws/cli/globals"
	"cloudctl/provider/aws/services/ec2"
	"context"
)

type eC2ListCmd struct {
	globals.AWSCLIFlag
	InstanceStates    []string `name:"state" help:"Return instance list of specific state(s) | values (pending | running | shutting-down | terminated | stopping | stopped)" default:""`
	InstanceTypes     []string `name:"type" help:"Return instance list of specific type(s) (for example, t2.micro)" default:""`
	AvailabilityZones []string `name:"az" help:"Return instance list of specific availability zone(s)" default:""`
	VpcIds            []string `name:"vpc" help:"Return instance list of specific vpcId(s)" default:""`
	SubnetIds         []string `name:"subnet" help:"Return instance list of specific subnet(s)" default:""`
	HasPublicIp       *bool    `name:"has-public-ip" help:"Return instance list which have public ip associate"`
	LaunchAtString    *string  `name:"launchat" help:"The time when the instance was launched, in the ISO 8601 format in the UTC time zone (YYYY-MM-DDThh:mm:ss.sssZ), for example, 2021-09-29T11:04:43.305Z. You can use a wildcard (*), for example, 2021-09-29T*, which matches an entire day."`
	FromSnapshot      bool     `name:"from-snapshot" help:"Read from the latest local snapshot (see 'ctl discover aws') instead of calling AWS live. Only --has-public-ip applies to snapshot data; other filters are AWS API-side and have no effect here."`
}

type instanceDefinitionCmd struct {
	globals.AWSCLIFlag
	Id string `name:"name" arg:"required"`
}

type ec2DescribeStatisticsCmd struct {
	globals.AWSCLIFlag
}

type sgExplainCmd struct {
	globals.AWSCLIFlag
	Id string `name:"name" arg:"required" help:"Security group ID (e.g. sg-0123456789abcdef0)"`
}

type EC2Command struct {
	List               eC2ListCmd               `name:"ls" cmd:"" help:"List ec2 instances"`
	InstacneDefinition instanceDefinitionCmd    `name:"def" cmd:"" help:"Get ec2 instance definition"`
	DescribeStatistics ec2DescribeStatisticsCmd `name:"stats" cmd:"" help:"Get ec2 instance(s) statistics"`
	Explain            sgExplainCmd             `name:"explain" cmd:"" help:"Explain a security group's rules using AI narration"`
}

func (cmd eC2ListCmd) Run(ctx context.Context, cli *global.CLIFlag) error {

	filters := []ec2.InstanceListFilterOptFunc{
		ec2.WithAvailabilityZone(cmd.AvailabilityZones),
		ec2.WithInstanceStates(cmd.InstanceStates),
		ec2.WithInstanceType(cmd.InstanceTypes),
		ec2.WithSubnetsIds(cmd.SubnetIds),
		ec2.WithVpcIds(cmd.VpcIds),
	}
	if cmd.HasPublicIp != nil {
		filters = append(filters, ec2.WithHasPublicIp())
	}
	if cmd.LaunchAtString != nil {
		filters = append(filters, ec2.WithLaunchAt(*cmd.LaunchAtString))
	}
	filter := ec2.NewInstanceFilter(filters...)

	if cmd.FromSnapshot {
		icmd, err := ec2.NewInstanceListFromSnapshotCommandExecutor(cli.TZShortIdentifier, *filter)
		if err != nil {
			return err
		}
		return icmd.Execute(ctx)
	}

	icmd, err := ec2.NewinstanceListCommandExecutor(&cmd.AWSCLIFlag, cli.TZShortIdentifier, *filter)
	if err != nil {
		return err
	}
	return icmd.Execute(ctx)
}

func (cmd instanceDefinitionCmd) Run(ctx context.Context, cli *global.CLIFlag) error {
	icmd, err := ec2.NewInstanceDescribeCommandExecutor(&cmd.AWSCLIFlag, cli.TZShortIdentifier, cmd.Id)
	if err != nil {
		return err
	}
	return icmd.Execute(ctx)
}

func (cmd ec2DescribeStatisticsCmd) Run(ctx context.Context, cli *global.CLIFlag) error {
	icmd, err := ec2.NewEC2StatisticsDescribeCommandExecutor(&cmd.AWSCLIFlag, cli.TZShortIdentifier)
	if err != nil {
		return err
	}
	return icmd.Execute(ctx)
}

func (cmd sgExplainCmd) Run(ctx context.Context) error {
	icmd, err := ec2.NewSecurityGroupExplainCommandExecutor(&cmd.AWSCLIFlag, cmd.Id)
	if err != nil {
		return err
	}
	return icmd.Execute(ctx)
}
